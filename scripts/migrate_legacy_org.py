#!/usr/bin/env python3
"""
Migrate legacy assess(2025) JSON objects into assessv2 org structure tables.

Source:
  - <legacy-dir>/collectives/*.json
  - <legacy-dir>/individuals/*.json

Target tables:
  - organizations
  - departments
  - employees

This script only migrates organizational structure data and does not import
scores/rules/calculation records.
"""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any


COLLECTIVE_TYPE_GROUP_DEPT = "集团部门"
COLLECTIVE_TYPE_COMPANY = "权属企业"

REQUIRED_POSITION_LEVEL_CODES = {
    "leadership_main",
    "leadership_deputy",
    "department_main",
    "department_deputy",
    "general_management_personnel",
}

INDIVIDUAL_TYPE_TO_POSITION_LEVEL = {
    "权属企业正职": "leadership_main",
    "权属企业副职": "leadership_deputy",
    "集团部门正职": "department_main",
    "集团部门副职": "department_deputy",
    "集团一般人员": "general_management_personnel",
    "集团其他": "general_management_personnel",
    "集团其他高管": "leadership_deputy",
}


@dataclass
class ImportStats:
    org_created: int = 0
    org_existing: int = 0
    dept_created: int = 0
    dept_existing: int = 0
    emp_created: int = 0
    emp_existing: int = 0
    skipped_collectives: int = 0
    skipped_individuals: int = 0


@dataclass
class TargetRef:
    organization_id: int
    department_id: int | None


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(
        description="Import legacy JSON organization objects into assessv2 sqlite DB."
    )
    parser.add_argument(
        "--legacy-dir",
        default=r"D:\scripts\assess\data",
        help="Legacy data root directory containing collectives/ and individuals/.",
    )
    parser.add_argument(
        "--db-path",
        default=str(repo_root / "data" / "2026" / "assess.db"),
        help="Target assessv2 sqlite database path.",
    )
    parser.add_argument(
        "--group-name",
        default="集团",
        help="Name of top-level group organization to create/use.",
    )
    parser.add_argument(
        "--created-by",
        type=int,
        default=None,
        help="Optional user id to write into created_by / updated_by.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Execute migration in transaction and rollback at end.",
    )
    return parser.parse_args()


def load_json_list(folder: Path) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []
    if not folder.is_dir():
        raise FileNotFoundError(f"folder not found: {folder}")
    for path in sorted(folder.glob("*.json")):
        with path.open("r", encoding="utf-8-sig") as f:
            payload = json.load(f)
        if not isinstance(payload, dict):
            raise ValueError(f"invalid json object: {path}")
        payload["_source_file"] = path.name
        items.append(payload)
    return items


def require_business_tables(conn: sqlite3.Connection) -> None:
    required = {"organizations", "departments", "employees", "position_levels"}
    rows = conn.execute(
        "SELECT name FROM sqlite_master WHERE type = 'table'"
    ).fetchall()
    existing = {row[0] for row in rows}
    missing = sorted(required - existing)
    if missing:
        raise RuntimeError(f"target db missing required tables: {', '.join(missing)}")


def load_position_level_ids(conn: sqlite3.Connection) -> dict[str, int]:
    rows = conn.execute(
        "SELECT id, level_code FROM position_levels WHERE status = 'active'"
    ).fetchall()
    mapping = {str(code): int(level_id) for level_id, code in rows}
    missing = sorted(REQUIRED_POSITION_LEVEL_CODES - set(mapping))
    if missing:
        raise RuntimeError(
            "target db missing required position_levels: " + ", ".join(missing)
        )
    return mapping


def find_org(
    conn: sqlite3.Connection, org_name: str, org_type: str, parent_id: int | None
) -> int | None:
    if parent_id is None:
        row = conn.execute(
            """
            SELECT id
            FROM organizations
            WHERE org_name = ? AND org_type = ? AND parent_id IS NULL AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (org_name, org_type),
        ).fetchone()
    else:
        row = conn.execute(
            """
            SELECT id
            FROM organizations
            WHERE org_name = ? AND org_type = ? AND parent_id = ? AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (org_name, org_type, parent_id),
        ).fetchone()
    return int(row[0]) if row else None


def get_or_create_org(
    conn: sqlite3.Connection,
    *,
    org_name: str,
    org_type: str,
    parent_id: int | None,
    created_by: int | None,
    now_ts: int,
    stats: ImportStats,
) -> int:
    existing_id = find_org(conn, org_name=org_name, org_type=org_type, parent_id=parent_id)
    if existing_id is not None:
        stats.org_existing += 1
        return existing_id
    cur = conn.execute(
        """
        INSERT INTO organizations
            (org_name, org_type, parent_id, status, created_by, created_at, updated_by, updated_at)
        VALUES
            (?, ?, ?, 'active', ?, ?, ?, ?)
        """,
        (org_name, org_type, parent_id, created_by, now_ts, created_by, now_ts),
    )
    stats.org_created += 1
    return int(cur.lastrowid)


def find_department(
    conn: sqlite3.Connection, dept_name: str, organization_id: int, parent_dept_id: int | None
) -> int | None:
    if parent_dept_id is None:
        row = conn.execute(
            """
            SELECT id
            FROM departments
            WHERE dept_name = ? AND organization_id = ? AND parent_dept_id IS NULL AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (dept_name, organization_id),
        ).fetchone()
    else:
        row = conn.execute(
            """
            SELECT id
            FROM departments
            WHERE dept_name = ? AND organization_id = ? AND parent_dept_id = ? AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (dept_name, organization_id, parent_dept_id),
        ).fetchone()
    return int(row[0]) if row else None


def get_or_create_department(
    conn: sqlite3.Connection,
    *,
    dept_name: str,
    organization_id: int,
    parent_dept_id: int | None,
    created_by: int | None,
    now_ts: int,
    stats: ImportStats,
) -> int:
    existing_id = find_department(
        conn,
        dept_name=dept_name,
        organization_id=organization_id,
        parent_dept_id=parent_dept_id,
    )
    if existing_id is not None:
        stats.dept_existing += 1
        return existing_id
    cur = conn.execute(
        """
        INSERT INTO departments
            (dept_name, organization_id, parent_dept_id, status, created_by, created_at, updated_by, updated_at)
        VALUES
            (?, ?, ?, 'active', ?, ?, ?, ?)
        """,
        (dept_name, organization_id, parent_dept_id, created_by, now_ts, created_by, now_ts),
    )
    stats.dept_created += 1
    return int(cur.lastrowid)


def find_employee(
    conn: sqlite3.Connection,
    *,
    emp_name: str,
    organization_id: int,
    department_id: int | None,
    position_level_id: int,
) -> int | None:
    if department_id is None:
        row = conn.execute(
            """
            SELECT id
            FROM employees
            WHERE emp_name = ?
              AND organization_id = ?
              AND department_id IS NULL
              AND position_level_id = ?
              AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (emp_name, organization_id, position_level_id),
        ).fetchone()
    else:
        row = conn.execute(
            """
            SELECT id
            FROM employees
            WHERE emp_name = ?
              AND organization_id = ?
              AND department_id = ?
              AND position_level_id = ?
              AND deleted_at IS NULL
            ORDER BY id
            LIMIT 1
            """,
            (emp_name, organization_id, department_id, position_level_id),
        ).fetchone()
    return int(row[0]) if row else None


def get_or_create_employee(
    conn: sqlite3.Connection,
    *,
    emp_name: str,
    organization_id: int,
    department_id: int | None,
    position_level_id: int,
    position_title: str,
    created_by: int | None,
    now_ts: int,
    stats: ImportStats,
) -> int:
    existing_id = find_employee(
        conn,
        emp_name=emp_name,
        organization_id=organization_id,
        department_id=department_id,
        position_level_id=position_level_id,
    )
    if existing_id is not None:
        stats.emp_existing += 1
        return existing_id
    cur = conn.execute(
        """
        INSERT INTO employees
            (emp_name, organization_id, department_id, position_level_id, position_title, status, created_by, created_at, updated_by, updated_at)
        VALUES
            (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)
        """,
        (
            emp_name,
            organization_id,
            department_id,
            position_level_id,
            position_title,
            created_by,
            now_ts,
            created_by,
            now_ts,
        ),
    )
    stats.emp_created += 1
    return int(cur.lastrowid)


def normalize_name(text: Any) -> str:
    value = "" if text is None else str(text)
    return value.strip()


def migrate(args: argparse.Namespace) -> int:
    legacy_dir = Path(args.legacy_dir).expanduser().resolve()
    db_path = Path(args.db_path).expanduser().resolve()
    if not db_path.is_file():
        raise FileNotFoundError(f"target db not found: {db_path}")

    collectives = load_json_list(legacy_dir / "collectives")
    individuals = load_json_list(legacy_dir / "individuals")

    stats = ImportStats()
    warnings: list[str] = []
    now_ts = int(time.time())

    conn = sqlite3.connect(str(db_path))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")

    try:
        require_business_tables(conn)
        position_level_ids = load_position_level_ids(conn)
        conn.execute("BEGIN")

        group_org_id = get_or_create_org(
            conn,
            org_name=normalize_name(args.group_name) or "集团",
            org_type="group",
            parent_id=None,
            created_by=args.created_by,
            now_ts=now_ts,
            stats=stats,
        )

        collective_target_map: dict[str, TargetRef] = {}
        for item in sorted(collectives, key=lambda x: str(x.get("id", ""))):
            legacy_id = normalize_name(item.get("id"))
            name = normalize_name(item.get("name"))
            source_type = normalize_name(item.get("type"))
            src_file = normalize_name(item.get("_source_file"))

            if not legacy_id or not name or not source_type:
                stats.skipped_collectives += 1
                warnings.append(
                    f"skip collective {src_file}: missing id/name/type"
                )
                continue

            if source_type == COLLECTIVE_TYPE_COMPANY:
                company_org_id = get_or_create_org(
                    conn,
                    org_name=name,
                    org_type="company",
                    parent_id=group_org_id,
                    created_by=args.created_by,
                    now_ts=now_ts,
                    stats=stats,
                )
                collective_target_map[legacy_id] = TargetRef(
                    organization_id=company_org_id, department_id=None
                )
            elif source_type == COLLECTIVE_TYPE_GROUP_DEPT:
                dept_id = get_or_create_department(
                    conn,
                    dept_name=name,
                    organization_id=group_org_id,
                    parent_dept_id=None,
                    created_by=args.created_by,
                    now_ts=now_ts,
                    stats=stats,
                )
                collective_target_map[legacy_id] = TargetRef(
                    organization_id=group_org_id, department_id=dept_id
                )
            else:
                stats.skipped_collectives += 1
                warnings.append(
                    f"skip collective {legacy_id} ({src_file}): unsupported type {source_type!r}"
                )

        for item in sorted(individuals, key=lambda x: str(x.get("id", ""))):
            legacy_id = normalize_name(item.get("id"))
            name = normalize_name(item.get("name"))
            source_type = normalize_name(item.get("type"))
            belongs_to = normalize_name(item.get("belongs_to"))
            src_file = normalize_name(item.get("_source_file"))

            if not legacy_id or not name or not source_type:
                stats.skipped_individuals += 1
                warnings.append(
                    f"skip individual {src_file}: missing id/name/type"
                )
                continue

            level_code = INDIVIDUAL_TYPE_TO_POSITION_LEVEL.get(source_type)
            if not level_code:
                stats.skipped_individuals += 1
                warnings.append(
                    f"skip individual {legacy_id} ({src_file}): unsupported type {source_type!r}"
                )
                continue

            target = None
            if belongs_to:
                target = collective_target_map.get(belongs_to)
                if target is None:
                    warnings.append(
                        f"individual {legacy_id}: belongs_to={belongs_to!r} not found, fallback to group root"
                    )
            if target is None:
                target = TargetRef(organization_id=group_org_id, department_id=None)

            level_id = position_level_ids[level_code]
            get_or_create_employee(
                conn,
                emp_name=name,
                organization_id=target.organization_id,
                department_id=target.department_id,
                position_level_id=level_id,
                position_title=source_type,
                created_by=args.created_by,
                now_ts=now_ts,
                stats=stats,
            )

        if args.dry_run:
            conn.rollback()
        else:
            conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()

    mode = "DRY-RUN (rolled back)" if args.dry_run else "APPLY (committed)"
    print(f"mode: {mode}")
    print(f"legacy_dir: {legacy_dir}")
    print(f"db_path: {db_path}")
    print(f"group_name: {normalize_name(args.group_name) or '集团'}")
    print("----")
    print(f"organizations created: {stats.org_created}")
    print(f"organizations existing: {stats.org_existing}")
    print(f"departments created: {stats.dept_created}")
    print(f"departments existing: {stats.dept_existing}")
    print(f"employees created: {stats.emp_created}")
    print(f"employees existing: {stats.emp_existing}")
    print(f"collectives skipped: {stats.skipped_collectives}")
    print(f"individuals skipped: {stats.skipped_individuals}")
    if warnings:
        print("---- warnings ----")
        for line in warnings:
            print(f"- {line}")
    return 0


def main() -> int:
    args = parse_args()
    try:
        return migrate(args)
    except Exception as exc:  # pragma: no cover
        print(f"migration failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
