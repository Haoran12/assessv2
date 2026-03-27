#!/usr/bin/env python3
from __future__ import annotations

import argparse
import datetime as dt
import json
import sqlite3
from pathlib import Path
from typing import Any


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description="Apply prepared legacy business data candidates into session DB.")
    parser.add_argument(
        "--session-db",
        default=str(repo_root / "data" / "2025年集团考核" / "assess.db"),
        help="Session sqlite db path.",
    )
    parser.add_argument(
        "--candidates",
        default="",
        help="Path to candidate json file. If empty, use latest file in migration-backups.",
    )
    parser.add_argument(
        "--backup-dir",
        default="",
        help="Backup directory. Default: <session-db-dir>/migration-backups",
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="Apply upsert into DB. Default is dry-run.",
    )
    return parser.parse_args()


def load_candidates(path: Path) -> list[dict[str, Any]]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, list):
        raise RuntimeError("candidate file root must be list")
    rows: list[dict[str, Any]] = []
    for item in payload:
        if not isinstance(item, dict):
            continue
        assessment_id = int(item.get("assessmentId") or 0)
        period_code = str(item.get("periodCode") or "").strip().upper()
        object_id = int(item.get("objectId") or 0)
        module_key = str(item.get("moduleKey") or "").strip()
        if assessment_id <= 0 or object_id <= 0 or not period_code or not module_key:
            continue
        rows.append(
            {
                "assessmentId": assessment_id,
                "periodCode": period_code,
                "objectId": object_id,
                "moduleKey": module_key,
                "score": float(item.get("score") or 0.0),
                "detailJson": str(item.get("detailJson") or ""),
                "sourceKind": str(item.get("sourceKind") or ""),
                "legacyObjectId": str(item.get("legacyObjectId") or ""),
                "legacyObjectName": str(item.get("legacyObjectName") or ""),
                "legacyObjectType": str(item.get("legacyObjectType") or ""),
            }
        )
    return rows


def pick_latest_candidate_file(backup_dir: Path) -> Path:
    files = sorted(backup_dir.glob("legacy_business_module_score_candidates_*.json"))
    if not files:
        raise FileNotFoundError(f"no candidate files found in {backup_dir}")
    return files[-1]


def key_of(item: dict[str, Any]) -> tuple[int, str, int, str]:
    return (
        int(item["assessmentId"]),
        str(item["periodCode"]),
        int(item["objectId"]),
        str(item["moduleKey"]),
    )


def main() -> int:
    args = parse_args()
    session_db = Path(args.session_db).expanduser().resolve()
    if not session_db.is_file():
        raise FileNotFoundError(f"session db not found: {session_db}")

    backup_dir = (
        Path(args.backup_dir).expanduser().resolve()
        if str(args.backup_dir or "").strip()
        else session_db.parent / "migration-backups"
    )
    backup_dir.mkdir(parents=True, exist_ok=True)

    candidate_file = (
        Path(args.candidates).expanduser().resolve()
        if str(args.candidates or "").strip()
        else pick_latest_candidate_file(backup_dir)
    )
    if not candidate_file.is_file():
        raise FileNotFoundError(f"candidate file not found: {candidate_file}")

    candidates = load_candidates(candidate_file)
    if not candidates:
        raise RuntimeError("no valid candidate rows")

    conn = sqlite3.connect(str(session_db))
    try:
        existing_rows = conn.execute(
            """
            SELECT assessment_id, period_code, object_id, module_key, score, detail_json
            FROM assessment_object_module_scores
            """
        ).fetchall()
        existing = {
            (int(r[0]), str(r[1] or "").strip().upper(), int(r[2]), str(r[3] or "").strip()): (float(r[4] or 0.0), str(r[5] or ""))
            for r in existing_rows
        }

        insert_count = 0
        update_count = 0
        noop_count = 0
        affected_existing: list[dict[str, Any]] = []

        for row in candidates:
            k = key_of(row)
            old = existing.get(k)
            if old is None:
                insert_count += 1
                continue
            same_score = abs(old[0] - float(row["score"])) < 1e-9
            same_detail = (old[1] or "") == (str(row["detailJson"]) or "")
            if same_score and same_detail:
                noop_count += 1
            else:
                update_count += 1
            affected_existing.append(
                {
                    "assessmentId": k[0],
                    "periodCode": k[1],
                    "objectId": k[2],
                    "moduleKey": k[3],
                    "score": old[0],
                    "detailJson": old[1],
                }
            )

        print(f"mode: {'APPLY' if args.apply else 'DRY-RUN'}")
        print(f"session_db: {session_db}")
        print(f"candidate_file: {candidate_file}")
        print(f"candidate_rows: {len(candidates)}")
        print("---- plan ----")
        print(f"insert: {insert_count}")
        print(f"update: {update_count}")
        print(f"noop: {noop_count}")

        if not args.apply:
            return 0

        now_tag = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
        backup_old = backup_dir / f"legacy_business_scores_before_{now_tag}.json"
        backup_plan = backup_dir / f"legacy_business_apply_plan_{now_tag}.json"
        backup_old.write_text(json.dumps(affected_existing, ensure_ascii=False, indent=2), encoding="utf-8")
        backup_plan.write_text(json.dumps(candidates, ensure_ascii=False, indent=2), encoding="utf-8")

        now_ts = int(dt.datetime.now().timestamp())
        with conn:
            for row in candidates:
                conn.execute(
                    """
                    INSERT INTO assessment_object_module_scores (
                        assessment_id, period_code, object_id, module_key, score, detail_json,
                        created_by, created_at, updated_by, updated_at
                    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    ON CONFLICT(assessment_id, period_code, object_id, module_key)
                    DO UPDATE SET
                        score = excluded.score,
                        detail_json = excluded.detail_json,
                        updated_by = excluded.updated_by,
                        updated_at = excluded.updated_at
                    """,
                    (
                        int(row["assessmentId"]),
                        str(row["periodCode"]),
                        int(row["objectId"]),
                        str(row["moduleKey"]),
                        float(row["score"]),
                        str(row["detailJson"]),
                        None,
                        now_ts,
                        None,
                        now_ts,
                    ),
                )

        print("---- apply ----")
        print(f"backup_old: {backup_old}")
        print(f"backup_plan: {backup_plan}")
        return 0
    finally:
        conn.close()


if __name__ == "__main__":
    raise SystemExit(main())

