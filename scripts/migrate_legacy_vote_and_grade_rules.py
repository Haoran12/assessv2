#!/usr/bin/env python3
"""
Migrate legacy vote module config and grade rules into session scopedRules.

Focus:
1) Fill voteConfig for vote modules from legacy voteModuleSettings.
2) Rebuild grades from legacy gradeRules (including quota/quotaMode/customConditions).

Inputs:
- Legacy config.json (default: d:\\scripts\\assess\\config.json)
- Session DB path (default: data\\2025年集团考核\\assess.db)

Outputs:
- dry-run report by default
- update rule_files.content_json and rule file content on disk with --apply
"""

from __future__ import annotations

import argparse
import copy
import datetime as dt
import json
import re
import sqlite3
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


GROUP_TO_LEGACY_CATEGORY: dict[str, tuple[str, str]] = {
    # group_code -> (legacy_root_key, legacy_category_name)
    "dept": ("collectives", "集团部门"),
    "child_org": ("collectives", "权属企业"),
    "dept_main": ("individuals", "集团部门正职"),
    "dept_deputy": ("individuals", "集团部门副职"),
    "general_staff": ("individuals", "集团一般人员"),
    "child_leadership_main": ("individuals", "权属企业正职"),
    "child_leadership_deputy": ("individuals", "权属企业副职"),
}

QUARTERLY_CODES = {"Q1", "Q2", "Q3", "Q4"}
ROUNDING_REAL = "real"
ROUNDING_FLOOR = "floor"
ROUNDING_CEIL = "ceil"


@dataclass
class MigrationStats:
    scoped_touched: int = 0
    scoped_created: int = 0
    vote_modules_patched: int = 0
    vote_modules_added: int = 0
    grades_replaced: int = 0
    warnings: list[str] = field(default_factory=list)

    def warn(self, text: str) -> None:
        self.warnings.append(text)


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(
        description="Migrate legacy vote modules and grade rules into scopedRules."
    )
    parser.add_argument(
        "--legacy-config",
        default=r"d:\scripts\assess\config.json",
        help="Path to legacy config.json",
    )
    parser.add_argument(
        "--session-db",
        default=str(repo_root / "data" / "2025年集团考核" / "assess.db"),
        help="Path to session assess.db",
    )
    parser.add_argument(
        "--rule-id",
        type=int,
        default=0,
        help="Rule file id in session DB (0 = pick latest by updated_at,id).",
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="Apply changes to DB and rule file; default is dry-run.",
    )
    parser.add_argument(
        "--backup-dir",
        default="",
        help="Backup directory when --apply (default: <session-db-dir>/migration-backups).",
    )
    return parser.parse_args()


def load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8-sig") as f:
        return json.load(f)


def period_scope(period_code: str) -> str:
    code = str(period_code or "").strip().upper()
    return "quarterly" if code in QUARTERLY_CODES else "annual"


def as_float(value: Any, default: float = 0.0) -> float:
    try:
        if value is None:
            return default
        return float(value)
    except (TypeError, ValueError):
        return default


def normalize_rounding_mode(value: Any) -> str:
    text = str(value or "").strip().lower()
    if text == ROUNDING_CEIL:
        return ROUNDING_CEIL
    if text == ROUNDING_FLOOR:
        return ROUNDING_FLOOR
    return ROUNDING_REAL


def normalize_logic(value: Any) -> str:
    text = str(value or "").strip().lower()
    if text in {"or", "||"}:
        return "or"
    return "and"


def normalize_upper_operator(value: Any) -> str:
    return "<" if str(value or "").strip() == "<" else "<="


def normalize_lower_operator(value: Any) -> str:
    return ">" if str(value or "").strip() == ">" else ">="


def normalize_period_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    result: list[str] = []
    seen: set[str] = set()
    for item in value:
        code = str(item or "").strip().upper()
        if not code or code in seen:
            continue
        seen.add(code)
        result.append(code)
    return result


def normalize_group_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    result: list[str] = []
    seen: set[str] = set()
    for item in value:
        code = str(item or "").strip()
        if not code or code in seen:
            continue
        seen.add(code)
        result.append(code)
    return result


def lookup_scoped_rule_index(content: dict[str, Any], period_code: str, group_code: str) -> int:
    period = str(period_code or "").strip().upper()
    group = str(group_code or "").strip()
    scoped_rules = content.get("scopedRules")
    if not isinstance(scoped_rules, list):
        return -1
    for idx, row in enumerate(scoped_rules):
        if not isinstance(row, dict):
            continue
        periods = normalize_period_list(row.get("applicablePeriods"))
        groups = normalize_group_list(row.get("applicableObjectGroups"))
        if period in periods and group in groups:
            return idx
    return -1


def ensure_scoped_rule(
    content: dict[str, Any],
    period_code: str,
    group_code: str,
    stats: MigrationStats,
) -> dict[str, Any]:
    scoped_rules = content.setdefault("scopedRules", [])
    if not isinstance(scoped_rules, list):
        content["scopedRules"] = []
        scoped_rules = content["scopedRules"]

    idx = lookup_scoped_rule_index(content, period_code, group_code)
    if idx >= 0:
        row = scoped_rules[idx]
        if isinstance(row, dict):
            return row

    safe_period = re.sub(r"[^A-Za-z0-9_]+", "_", str(period_code or "X").upper())
    safe_group = re.sub(r"[^A-Za-z0-9_]+", "_", str(group_code or "group"))
    created = {
        "id": f"migrated_{safe_period}_{safe_group}",
        "applicablePeriods": [str(period_code or "").strip().upper()],
        "applicableObjectGroups": [str(group_code or "").strip()],
        "scoreModules": [],
        "grades": [],
    }
    scoped_rules.append(created)
    stats.scoped_created += 1
    return created


def build_vote_config_for_category(legacy_config: dict[str, Any], category_name: str, stats: MigrationStats) -> dict[str, Any]:
    settings = legacy_config.get("voteModuleSettings", {})
    vote_options = settings.get("voteOptions", [])
    voter_types = settings.get("voterTypes", [])
    voter_type_weights = settings.get("voterTypeWeights", {})

    grade_scores: list[dict[str, Any]] = []
    if isinstance(vote_options, list):
        for row in vote_options:
            if not isinstance(row, dict):
                continue
            label = str(row.get("name") or row.get("label") or "").strip()
            if not label:
                continue
            score = as_float(row.get("score"), 0.0)
            grade_scores.append({"label": label, "score": score})

    subjects_raw: list[dict[str, Any]] = []
    if isinstance(voter_types, list):
        for row in voter_types:
            if not isinstance(row, dict):
                continue
            voter_id = str(row.get("id") or row.get("name") or "").strip()
            if not voter_id:
                continue
            voter_label = str(row.get("name") or voter_id).strip() or voter_id
            base_weight = as_float(row.get("weight"), 0.0)
            if base_weight <= 0:
                continue
            cat_factor = 1.0
            by_type = voter_type_weights.get(voter_id)
            if isinstance(by_type, dict):
                if category_name in by_type:
                    cat_factor = as_float(by_type.get(category_name), 1.0)
                else:
                    stats.warn(
                        f"voterTypeWeights[{voter_id}] missing category {category_name}, fallback factor=1"
                    )
            combined = base_weight * cat_factor
            if combined <= 0:
                continue
            subjects_raw.append({"label": voter_label, "weight": combined})

    if not subjects_raw:
        stats.warn(
            f"cannot derive voter subjects from voterTypes for {category_name}, fallback to single subject"
        )
        subjects_raw = [{"label": "主体1", "weight": 1.0}]

    total_weight = sum(as_float(item.get("weight"), 0.0) for item in subjects_raw)
    if total_weight <= 0:
        subjects = [{"label": "主体1", "weight": 1.0}]
    else:
        subjects = []
        for item in subjects_raw:
            label = str(item.get("label") or "").strip() or "主体"
            normalized_weight = as_float(item.get("weight"), 0.0) / total_weight
            subjects.append({"label": label, "weight": normalized_weight})

    return {
        "gradeScores": grade_scores,
        "voterSubjects": subjects,
    }


def normalize_module_key(row: dict[str, Any]) -> str:
    return str(row.get("moduleKey") or row.get("id") or "").strip()


def patch_vote_modules(
    scoped_rule: dict[str, Any],
    legacy_modules: list[dict[str, Any]],
    vote_config: dict[str, Any],
    stats: MigrationStats,
) -> None:
    modules = scoped_rule.get("scoreModules")
    if not isinstance(modules, list):
        scoped_rule["scoreModules"] = []
        modules = scoped_rule["scoreModules"]

    existing_by_key: dict[str, dict[str, Any]] = {}
    for row in modules:
        if isinstance(row, dict):
            key = normalize_module_key(row)
            if key:
                existing_by_key[key] = row

    legacy_vote_modules = []
    for row in legacy_modules:
        if not isinstance(row, dict):
            continue
        method = str(row.get("calculationMethod") or "").strip().lower()
        if method == "vote":
            legacy_vote_modules.append(row)

    patched_vote_keys: set[str] = set()

    def mark_vote_patched(module_key: str) -> None:
        key = str(module_key or "").strip()
        if not key or key in patched_vote_keys:
            return
        patched_vote_keys.add(key)
        stats.vote_modules_patched += 1

    # Patch existing vote modules first.
    for row in modules:
        if not isinstance(row, dict):
            continue
        method = str(row.get("calculationMethod") or "").strip().lower()
        if method != "vote":
            continue
        row["calculationMethod"] = "vote"
        detail = row.get("detail")
        if not isinstance(detail, dict):
            detail = {}
        detail["voteConfig"] = copy.deepcopy(vote_config)
        row["detail"] = detail
        row["voteConfig"] = copy.deepcopy(vote_config)
        mark_vote_patched(normalize_module_key(row))

    # Add/patch vote modules from legacy definition.
    for legacy_row in legacy_vote_modules:
        key = str(legacy_row.get("id") or legacy_row.get("moduleKey") or "").strip()
        if not key:
            continue
        module = existing_by_key.get(key)
        if module is None:
            module = {
                "id": key,
                "moduleKey": key,
                "moduleName": str(legacy_row.get("name") or legacy_row.get("moduleName") or key).strip() or key,
                "weight": as_float(legacy_row.get("weight"), 0.0),
                "calculationMethod": "vote",
                "customScript": "",
            }
            modules.append(module)
            existing_by_key[key] = module
            stats.vote_modules_added += 1
        module["calculationMethod"] = "vote"
        if as_float(module.get("weight"), 0.0) <= 0:
            module["weight"] = as_float(legacy_row.get("weight"), 0.0)
        detail = module.get("detail")
        if not isinstance(detail, dict):
            detail = {}
        detail["voteConfig"] = copy.deepcopy(vote_config)
        module["detail"] = detail
        module["voteConfig"] = copy.deepcopy(vote_config)
        mark_vote_patched(key)


def to_expr_script(script: str) -> str:
    text = str(script or "").replace("\r\n", "\n").replace("\r", "\n")
    lines = []
    for raw in text.split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "#" in line:
            line = line.split("#", 1)[0].strip()
        if line:
            lines.append(line)
    if not lines:
        return ""

    assign_map: dict[str, str] = {}
    result_expr = ""
    assign_pattern = re.compile(r"^([A-Za-z_]\w*)\s*=\s*(.+)$")
    for line in lines:
        m = assign_pattern.match(line)
        if not m:
            result_expr = line
            continue
        name = m.group(1)
        expr = m.group(2).strip()
        if name == "result":
            result_expr = expr
        else:
            assign_map[name] = expr

    if not result_expr:
        result_expr = lines[-1]

    def translate(expr_text: str) -> str:
        x = expr_text.strip()
        x = re.sub(
            r"self\.get_grade\(\s*['\"]([^'\"]+)['\"]\s*\)",
            lambda m: f'grade("{m.group(1).strip().upper()}", objectId)',
            x,
            flags=re.IGNORECASE,
        )
        x = re.sub(
            r"self\.get_score\(\s*['\"]([^'\"]+)['\"]\s*,\s*['\"]([^'\"]+)['\"]\s*\)",
            lambda m: f'moduleScore("{m.group(1).strip().upper()}", objectId, "{m.group(2).strip()}")',
            x,
            flags=re.IGNORECASE,
        )
        x = re.sub(r"\bTrue\b", "true", x)
        x = re.sub(r"\bFalse\b", "false", x)
        x = re.sub(r"\band\b", "&&", x, flags=re.IGNORECASE)
        x = re.sub(r"\bor\b", "||", x, flags=re.IGNORECASE)
        x = re.sub(r"\bnot\b", "!", x, flags=re.IGNORECASE)
        return x.strip()

    expr = translate(result_expr)
    # Inline simple assignment variables into final expression.
    for name in sorted(assign_map.keys(), key=len, reverse=True):
        value_expr = translate(assign_map[name])
        expr = re.sub(rf"\b{name}\b", f"({value_expr})", expr)

    expr = re.sub(r"\s+", " ", expr).strip()
    return expr


def convert_custom_conditions(custom_conditions: Any) -> tuple[bool, str, str]:
    if not isinstance(custom_conditions, list) or not custom_conditions:
        return False, "", "and"

    exprs: list[str] = []
    join_logic = "and"
    for idx, row in enumerate(custom_conditions):
        if not isinstance(row, dict):
            continue
        raw_rule = str(row.get("rule") or "").strip()
        if not raw_rule:
            continue
        if idx == 0:
            join_logic = normalize_logic(row.get("logic"))
        expr = to_expr_script(raw_rule)
        if expr:
            exprs.append(expr)

    if not exprs:
        return False, "", "and"
    if len(exprs) == 1:
        return True, exprs[0], "and"

    op = " && " if join_logic == "and" else " || "
    combined = op.join(f"({item})" for item in exprs)
    return True, combined, "and"


def convert_structured_grades(legacy_rules: list[dict[str, Any]]) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    for idx, row in enumerate(legacy_rules):
        if not isinstance(row, dict):
            continue
        title = str(row.get("grade") or row.get("title") or f"等第{idx + 1}").strip()
        score_cfg = row.get("totalScoreConstraint")
        if not isinstance(score_cfg, dict):
            score_cfg = {}
        min_value = score_cfg.get("min", None)
        max_value = score_cfg.get("max", None)
        has_lower = min_value is not None
        has_upper = max_value is not None
        max_ratio = row.get("quota")
        max_ratio_percent = None
        if max_ratio is not None:
            max_ratio_percent = as_float(max_ratio, 0.0) * 100.0
        extra_enabled, extra_script, condition_logic = convert_custom_conditions(row.get("customConditions"))
        converted = {
            "id": f"grade_{idx + 1}",
            "title": title,
            "scoreNode": {
                "hasUpperLimit": has_upper,
                "upperScore": as_float(max_value, 0.0) if has_upper else 0.0,
                "upperOperator": normalize_upper_operator(score_cfg.get("maxOp")),
                "hasLowerLimit": has_lower,
                "lowerScore": as_float(min_value, 0.0) if has_lower else 0.0,
                "lowerOperator": normalize_lower_operator(score_cfg.get("minOp")),
            },
            "extraConditionEnabled": extra_enabled,
            "extraConditionScript": extra_script,
            "conditionLogic": condition_logic,
            "maxRatioPercent": max_ratio_percent,
            "maxRatioRoundingMode": normalize_rounding_mode(row.get("quotaMode")),
        }
        result.append(converted)
    return result


def convert_min_score_grades(legacy_rules: list[dict[str, Any]]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for idx, row in enumerate(legacy_rules):
        if not isinstance(row, dict):
            continue
        if row.get("minScore") is None:
            continue
        rows.append(
            {
                "idx": idx,
                "title": str(row.get("grade") or row.get("title") or f"等第{idx + 1}").strip(),
                "minScore": as_float(row.get("minScore"), 0.0),
                "quota": row.get("quota"),
                "quotaMode": row.get("quotaMode"),
                "customConditions": row.get("customConditions"),
            }
        )
    if not rows:
        return []

    rows.sort(key=lambda x: (-x["minScore"], x["idx"]))
    result: list[dict[str, Any]] = []
    for idx, row in enumerate(rows):
        lower = row["minScore"]
        upper = 100.0 if idx == 0 else rows[idx - 1]["minScore"]
        upper_op = "<=" if idx == 0 else "<"
        max_ratio_percent = None
        if row["quota"] is not None:
            max_ratio_percent = as_float(row["quota"], 0.0) * 100.0
        extra_enabled, extra_script, condition_logic = convert_custom_conditions(row.get("customConditions"))
        converted = {
            "id": f"grade_{idx + 1}",
            "title": row["title"] or f"等第{idx + 1}",
            "scoreNode": {
                "hasUpperLimit": True,
                "upperScore": as_float(upper, 0.0),
                "upperOperator": upper_op,
                "hasLowerLimit": True,
                "lowerScore": as_float(lower, 0.0),
                "lowerOperator": ">=",
            },
            "extraConditionEnabled": extra_enabled,
            "extraConditionScript": extra_script,
            "conditionLogic": condition_logic,
            "maxRatioPercent": max_ratio_percent,
            "maxRatioRoundingMode": normalize_rounding_mode(row.get("quotaMode")),
        }
        result.append(converted)
    return result


def convert_grade_rules(legacy_rules: Any) -> list[dict[str, Any]]:
    if not isinstance(legacy_rules, list) or not legacy_rules:
        return []
    has_structured = any(
        isinstance(row, dict) and isinstance(row.get("totalScoreConstraint"), dict)
        for row in legacy_rules
    )
    if has_structured:
        return convert_structured_grades([row for row in legacy_rules if isinstance(row, dict)])
    return convert_min_score_grades([row for row in legacy_rules if isinstance(row, dict)])


def get_legacy_modules(legacy_config: dict[str, Any], scope_key: str, root_key: str, category_name: str) -> list[dict[str, Any]]:
    score_modules = legacy_config.get("scoreModules", {})
    scope_node = score_modules.get(scope_key, {})
    root_node = scope_node.get(root_key, {})
    modules = root_node.get(category_name, [])
    if not isinstance(modules, list):
        return []
    return [row for row in modules if isinstance(row, dict)]


def get_legacy_grades(legacy_config: dict[str, Any], root_key: str, category_name: str) -> list[dict[str, Any]]:
    grade_rules = legacy_config.get("gradeRules", {})
    root_node = grade_rules.get(root_key, {})
    rows = root_node.get(category_name, [])
    if not isinstance(rows, list):
        return []
    return [row for row in rows if isinstance(row, dict)]


def migrate_content(
    content: dict[str, Any],
    legacy_config: dict[str, Any],
    periods: list[str],
    groups: list[str],
) -> tuple[dict[str, Any], MigrationStats]:
    migrated = copy.deepcopy(content)
    migrated["version"] = max(int(migrated.get("version") or 0), 3)
    stats = MigrationStats()
    touched_keys: set[str] = set()

    for period_code in periods:
        scope_key = period_scope(period_code)
        for group_code in groups:
            mapping = GROUP_TO_LEGACY_CATEGORY.get(group_code)
            if mapping is None:
                continue
            root_key, category_name = mapping
            legacy_modules = get_legacy_modules(legacy_config, scope_key, root_key, category_name)
            legacy_grades = get_legacy_grades(legacy_config, root_key, category_name)
            if not legacy_modules and not legacy_grades:
                continue

            scoped = ensure_scoped_rule(migrated, period_code, group_code, stats)
            key = f"{period_code}|{group_code}"
            if key not in touched_keys:
                touched_keys.add(key)
                stats.scoped_touched += 1

            vote_config = build_vote_config_for_category(legacy_config, category_name, stats)
            patch_vote_modules(scoped, legacy_modules, vote_config, stats)

            converted_grades = convert_grade_rules(legacy_grades)
            if converted_grades:
                scoped["grades"] = converted_grades
                stats.grades_replaced += 1

    return migrated, stats


def find_rule_record(conn: sqlite3.Connection, rule_id: int) -> tuple[int, str, str]:
    cur = conn.cursor()
    if rule_id > 0:
        row = cur.execute(
            "SELECT id, content_json, file_path FROM rule_files WHERE id = ?",
            (rule_id,),
        ).fetchone()
    else:
        row = cur.execute(
            "SELECT id, content_json, file_path FROM rule_files ORDER BY updated_at DESC, id DESC LIMIT 1"
        ).fetchone()
    if not row:
        raise RuntimeError("rule_files is empty")
    return int(row[0]), str(row[1] or ""), str(row[2] or "")


def list_period_codes(conn: sqlite3.Connection) -> list[str]:
    cur = conn.cursor()
    rows = cur.execute(
        "SELECT period_code FROM assessment_session_periods ORDER BY sort_order ASC, id ASC"
    ).fetchall()
    result = []
    for row in rows:
        code = str(row[0] or "").strip().upper()
        if code:
            result.append(code)
    return result


def list_group_codes(conn: sqlite3.Connection) -> list[str]:
    cur = conn.cursor()
    rows = cur.execute(
        "SELECT group_code FROM assessment_object_groups ORDER BY sort_order ASC, id ASC"
    ).fetchall()
    result = []
    for row in rows:
        code = str(row[0] or "").strip()
        if code:
            result.append(code)
    return result


def ensure_backup_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def apply_updates(
    conn: sqlite3.Connection,
    rule_id: int,
    migrated_json_text: str,
    rule_file_path: str,
    backup_dir: Path,
    old_json_text: str,
) -> tuple[Path, Path, Path | None]:
    now_tag = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
    ensure_backup_dir(backup_dir)
    backup_old = backup_dir / f"rule_{rule_id}_before_{now_tag}.json"
    backup_new = backup_dir / f"rule_{rule_id}_after_{now_tag}.json"
    write_text(backup_old, old_json_text)
    write_text(backup_new, migrated_json_text)

    cur = conn.cursor()
    cur.execute(
        "UPDATE rule_files SET content_json = ?, updated_at = strftime('%s','now') WHERE id = ?",
        (migrated_json_text, rule_id),
    )
    conn.commit()

    file_backup: Path | None = None
    path_text = str(rule_file_path or "").strip()
    if path_text:
        target = Path(path_text)
        if target.exists():
            file_backup = backup_dir / f"rule_{rule_id}_file_before_{now_tag}.json"
            write_text(file_backup, target.read_text(encoding="utf-8"))
        write_text(target, migrated_json_text)

    return backup_old, backup_new, file_backup


def main() -> int:
    args = parse_args()
    legacy_config_path = Path(args.legacy_config).expanduser().resolve()
    session_db_path = Path(args.session_db).expanduser().resolve()

    if not legacy_config_path.is_file():
        print(f"legacy config not found: {legacy_config_path}", file=sys.stderr)
        return 1
    if not session_db_path.is_file():
        print(f"session db not found: {session_db_path}", file=sys.stderr)
        return 1

    legacy_config = load_json(legacy_config_path)
    if not isinstance(legacy_config, dict):
        print("legacy config json root must be object", file=sys.stderr)
        return 1

    conn = sqlite3.connect(str(session_db_path))
    conn.row_factory = sqlite3.Row
    try:
        rule_id, content_json_text, rule_file_path = find_rule_record(conn, int(args.rule_id))
        content_obj = json.loads(content_json_text or "{}")
        if not isinstance(content_obj, dict):
            print("rule content_json root must be object", file=sys.stderr)
            return 1

        periods = list_period_codes(conn)
        groups = list_group_codes(conn)
        migrated_obj, stats = migrate_content(content_obj, legacy_config, periods, groups)
        migrated_json_text = json.dumps(migrated_obj, ensure_ascii=False, indent=2)

        print(f"mode: {'APPLY' if args.apply else 'DRY-RUN'}")
        print(f"legacy_config: {legacy_config_path}")
        print(f"session_db: {session_db_path}")
        print(f"rule_id: {rule_id}")
        print(f"rule_file_path: {rule_file_path}")
        print("----")
        print(f"periods: {periods}")
        print(f"groups: {groups}")
        print("---- summary ----")
        print(f"scoped_touched: {stats.scoped_touched}")
        print(f"scoped_created: {stats.scoped_created}")
        print(f"vote_modules_patched: {stats.vote_modules_patched}")
        print(f"vote_modules_added: {stats.vote_modules_added}")
        print(f"grades_replaced: {stats.grades_replaced}")
        print(f"warnings: {len(stats.warnings)}")
        if stats.warnings:
            print("---- warning details ----")
            for item in stats.warnings:
                print(f"- {item}")

        if not args.apply:
            return 0

        backup_dir = (
            Path(args.backup_dir).expanduser().resolve()
            if str(args.backup_dir or "").strip()
            else session_db_path.parent / "migration-backups"
        )
        backup_old, backup_new, file_backup = apply_updates(
            conn=conn,
            rule_id=rule_id,
            migrated_json_text=migrated_json_text,
            rule_file_path=rule_file_path,
            backup_dir=backup_dir,
            old_json_text=content_json_text,
        )
        print("---- apply ----")
        print(f"backup_old: {backup_old}")
        print(f"backup_new: {backup_new}")
        if file_backup is not None:
            print(f"rule_file_backup: {file_backup}")
        return 0
    except Exception as exc:
        print(f"migration failed: {exc}", file=sys.stderr)
        return 1
    finally:
        conn.close()


if __name__ == "__main__":
    raise SystemExit(main())
