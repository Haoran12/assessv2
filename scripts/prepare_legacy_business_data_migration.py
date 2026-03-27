#!/usr/bin/env python3
from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Any

TYPE_TO_GROUP = {
    "集团部门": "dept",
    "权属企业": "child_org",
    "集团部门正职": "dept_main",
    "集团部门副职": "dept_deputy",
    "集团一般人员": "general_staff",
    "权属企业正职": "child_leadership_main",
    "权属企业副职": "child_leadership_deputy",
    "集团其他高管": "general_staff",
}

PERIOD_MAP = {
    "Q1": "Q1",
    "Q2": "Q2",
    "Q3": "Q3",
    "Q4": "Q4",
    "ANNUAL": "ANNAUL",
    "ANNAUL": "ANNAUL",
}


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    p = argparse.ArgumentParser(description="Prepare legacy business data mapping and migration candidates")
    p.add_argument("--legacy-dir", default=r"D:\scripts\assess\data")
    p.add_argument("--session-db", default=str(repo_root / "data" / "2025年集团考核" / "assess.db"))
    p.add_argument("--rule-id", type=int, default=0)
    p.add_argument("--output-dir", default="")
    return p.parse_args()


def norm_name(value: Any) -> str:
    return re.sub(r"\s+", "", str(value or ""))


def as_float(value: Any, default: float = 0.0) -> float:
    try:
        if value is None:
            return default
        return float(value)
    except (TypeError, ValueError):
        return default


def load_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8-sig") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"invalid json object: {path}")
    return data


def find_rule(conn: sqlite3.Connection, rule_id: int) -> tuple[int, dict[str, Any]]:
    cur = conn.cursor()
    if rule_id > 0:
        row = cur.execute("SELECT id, content_json FROM rule_files WHERE id = ?", (rule_id,)).fetchone()
    else:
        row = cur.execute("SELECT id, content_json FROM rule_files ORDER BY updated_at DESC, id DESC LIMIT 1").fetchone()
    if not row:
        raise RuntimeError("no rule_files found")
    content = json.loads(str(row[1] or "{}"))
    if not isinstance(content, dict):
        raise RuntimeError("rule content_json root must be object")
    return int(row[0]), content


def build_rule_index(rule: dict[str, Any]) -> tuple[dict[tuple[str, str], set[str]], dict[tuple[str, str, str], dict[str, Any]]]:
    modules_by_scope: dict[tuple[str, str], set[str]] = {}
    node_by_scope_module: dict[tuple[str, str, str], dict[str, Any]] = {}
    scoped_rules = rule.get("scopedRules")
    if not isinstance(scoped_rules, list):
        return modules_by_scope, node_by_scope_module
    for scoped in scoped_rules:
        if not isinstance(scoped, dict):
            continue
        periods = scoped.get("applicablePeriods")
        groups = scoped.get("applicableObjectGroups")
        if not isinstance(periods, list) or not isinstance(groups, list):
            continue
        period_codes = [str(x or "").strip().upper() for x in periods if str(x or "").strip()]
        group_codes = [str(x or "").strip() for x in groups if str(x or "").strip()]
        if not period_codes or not group_codes:
            continue
        score_modules = scoped.get("scoreModules")
        if not isinstance(score_modules, list):
            score_modules = []
        for p in period_codes:
            for g in group_codes:
                key = (p, g)
                modules_by_scope.setdefault(key, set())
                for mod in score_modules:
                    if not isinstance(mod, dict):
                        continue
                    mk = str(mod.get("moduleKey") or mod.get("id") or "").strip()
                    if not mk:
                        continue
                    modules_by_scope[key].add(mk)
                    node_by_scope_module[(p, g, mk)] = mod
    return modules_by_scope, node_by_scope_module


def parse_vote_config(module_node: dict[str, Any]) -> dict[str, Any]:
    top = module_node.get("voteConfig")
    if isinstance(top, dict) and top:
        return top
    detail = module_node.get("detail")
    if isinstance(detail, dict):
        for k in ("voteConfig", "vote", "voteDetail"):
            v = detail.get(k)
            if isinstance(v, dict) and v:
                return v
        if detail:
            return detail
    return {}


def build_vote_input(legacy_raw: Any) -> dict[str, Any]:
    if not isinstance(legacy_raw, dict):
        raise ValueError("legacy raw vote must be object")
    subject_votes: list[dict[str, Any]] = []
    for subject_label, grade_map in legacy_raw.items():
        s = str(subject_label or "").strip()
        if not s:
            continue
        grade_votes: list[dict[str, Any]] = []
        if isinstance(grade_map, dict):
            for grade_label, count in grade_map.items():
                g = str(grade_label or "").strip()
                if not g:
                    continue
                grade_votes.append({"gradeLabel": g, "count": as_float(count, 0.0)})
        subject_votes.append({"subjectLabel": s, "gradeVotes": grade_votes})
    return {"subjectVotes": subject_votes}


def calc_vote_score(vote_config: dict[str, Any], vote_input: dict[str, Any]) -> tuple[float, dict[str, Any]]:
    grade_rows = vote_config.get("gradeScores")
    subject_rows = vote_config.get("voterSubjects")
    if not isinstance(grade_rows, list) or not isinstance(subject_rows, list):
        raise ValueError("vote config missing gradeScores/voterSubjects")

    grade_scores = []
    for r in grade_rows:
        if not isinstance(r, dict):
            continue
        label = str(r.get("label") or "").strip()
        if label:
            grade_scores.append({"label": label, "score": as_float(r.get("score"), 0.0)})
    if not grade_scores:
        raise ValueError("empty vote gradeScores")

    subjects = []
    for r in subject_rows:
        if not isinstance(r, dict):
            continue
        label = str(r.get("label") or "").strip()
        weight = as_float(r.get("weight"), 0.0)
        if label and weight > 0:
            subjects.append({"label": label, "weight": weight})
    if not subjects:
        raise ValueError("empty vote voterSubjects")

    sv = vote_input.get("subjectVotes")
    if not isinstance(sv, list) or not sv:
        raise ValueError("vote input subjectVotes is empty")

    grade_score_by_label = {r["label"]: r["score"] for r in grade_scores}
    subject_weight_by_label = {r["label"]: r["weight"] for r in subjects}

    input_by_subject: dict[str, dict[str, float]] = {}
    for row in sv:
        if not isinstance(row, dict):
            continue
        sl = str(row.get("subjectLabel") or "").strip()
        if not sl:
            raise ValueError("empty subjectLabel")
        if sl not in subject_weight_by_label:
            raise ValueError(f"subjectLabel not in config: {sl}")
        if sl in input_by_subject:
            raise ValueError(f"duplicate subjectLabel: {sl}")
        gm: dict[str, float] = {}
        for gv in row.get("gradeVotes") or []:
            if not isinstance(gv, dict):
                continue
            gl = str(gv.get("gradeLabel") or "").strip()
            if not gl:
                continue
            if gl not in grade_score_by_label:
                raise ValueError(f"gradeLabel not in config: {gl}")
            cnt = as_float(gv.get("count"), 0.0)
            if cnt < 0:
                raise ValueError(f"negative vote count: {sl}/{gl}")
            gm[gl] = round(cnt)
        input_by_subject[sl] = gm

    total_score = 0.0
    has_any_vote = False
    subject_details: list[dict[str, Any]] = []
    for subject in subjects:
        sl = subject["label"]
        sw = subject["weight"]
        count_map = input_by_subject.get(sl, {})
        subject_total = 0.0
        for g in grade_scores:
            subject_total += count_map.get(g["label"], 0.0)
        contribution = 0.0
        if subject_total > 0:
            has_any_vote = True
            for g in grade_scores:
                cnt = count_map.get(g["label"], 0.0)
                if cnt > 0:
                    contribution += g["score"] * sw * (cnt / subject_total)
        total_score += contribution
        subject_details.append(
            {
                "subjectLabel": sl,
                "subjectWeight": sw,
                "subjectTotalVotes": subject_total,
                "scoreContribution": contribution,
            }
        )
    if not has_any_vote:
        raise ValueError("vote input has no positive votes")

    detail = {
        "type": "vote_weighted_rate_sum",
        "calculatedScore": total_score,
        "voteConfig": {
            "gradeScores": [{"label": r["label"], "score": r["score"]} for r in grade_scores],
            "voterSubjects": [{"label": r["label"], "weight": r["weight"]} for r in subjects],
        },
        "voteInput": vote_input,
        "subjectDetails": subject_details,
    }
    return total_score, detail


def main() -> int:
    args = parse_args()
    legacy_dir = Path(args.legacy_dir).expanduser().resolve()
    session_db = Path(args.session_db).expanduser().resolve()
    if not (legacy_dir / "collectives").is_dir() or not (legacy_dir / "individuals").is_dir():
        print(f"legacy dir invalid: {legacy_dir}")
        return 1
    if not session_db.is_file():
        print(f"session db not found: {session_db}")
        return 1

    collectives = [load_json(p) for p in sorted((legacy_dir / "collectives").glob("*.json"))]
    individuals = [load_json(p) for p in sorted((legacy_dir / "individuals").glob("*.json"))]

    conn = sqlite3.connect(str(session_db))
    try:
        rule_id, rule = find_rule(conn, args.rule_id)
        modules_by_scope, node_by_scope_module = build_rule_index(rule)

        object_rows = conn.execute(
            """
            SELECT o.id, o.assessment_id, o.object_name, o.group_code, o.object_type, o.target_type, o.target_id,
                   o.parent_object_id, p.object_name
            FROM assessment_session_objects o
            LEFT JOIN assessment_session_objects p ON p.id = o.parent_object_id
            ORDER BY o.id
            """
        ).fetchall()
        objects = [
            {
                "id": int(r[0]),
                "assessment_id": int(r[1]),
                "name": str(r[2] or ""),
                "group_code": str(r[3] or ""),
                "object_type": str(r[4] or ""),
                "target_type": str(r[5] or ""),
                "target_id": int(r[6] or 0),
                "parent_object_id": int(r[7]) if r[7] is not None else None,
                "parent_name": str(r[8] or ""),
                "name_norm": norm_name(r[2]),
            }
            for r in object_rows
        ]
        if not objects:
            raise RuntimeError("assessment_session_objects is empty")
        assessment_id = int(objects[0]["assessment_id"])

        teams = [o for o in objects if o["object_type"] == "team"]
        team_by_name: dict[str, list[dict[str, Any]]] = {}
        for t in teams:
            team_by_name.setdefault(t["name_norm"], []).append(t)

        mapped_collectives: dict[str, dict[str, Any]] = {}
        collective_unmatched: list[dict[str, Any]] = []
        collective_ambiguous: list[dict[str, Any]] = []

        for row in collectives:
            legacy_id = str(row.get("id") or "")
            legacy_name = str(row.get("name") or "")
            legacy_type = str(row.get("type") or "")
            expected_group = TYPE_TO_GROUP.get(legacy_type, "")
            cands = team_by_name.get(norm_name(legacy_name), [])
            if expected_group:
                filtered = [c for c in cands if c["group_code"] == expected_group]
                if filtered:
                    cands = filtered
            if len(cands) == 1:
                mapped_collectives[legacy_id] = cands[0]
            elif len(cands) == 0:
                collective_unmatched.append(
                    {
                        "legacyId": legacy_id,
                        "legacyName": legacy_name,
                        "legacyType": legacy_type,
                        "expectedGroupCode": expected_group,
                        "reason": "no_session_team_match_by_name",
                    }
                )
            else:
                collective_ambiguous.append(
                    {
                        "legacyId": legacy_id,
                        "legacyName": legacy_name,
                        "legacyType": legacy_type,
                        "expectedGroupCode": expected_group,
                        "candidateObjectIds": [c["id"] for c in cands],
                    }
                )

        session_individuals = [o for o in objects if o["object_type"] == "individual"]
        ind_by_name: dict[str, list[dict[str, Any]]] = {}
        for i in session_individuals:
            ind_by_name.setdefault(i["name_norm"], []).append(i)

        mapped_individuals: list[dict[str, Any]] = []
        individual_unmatched: list[dict[str, Any]] = []
        individual_parent_mismatch: list[dict[str, Any]] = []
        individual_group_mismatch: list[dict[str, Any]] = []

        for row in individuals:
            legacy_id = str(row.get("id") or "")
            legacy_name = str(row.get("name") or "")
            legacy_type = str(row.get("type") or "")
            expected_group = TYPE_TO_GROUP.get(legacy_type, "")
            cands = ind_by_name.get(norm_name(legacy_name), [])
            if expected_group:
                filtered = [c for c in cands if c["group_code"] == expected_group]
                if filtered:
                    cands = filtered

            picked: dict[str, Any] | None = None
            if len(cands) == 1:
                picked = cands[0]
            elif len(cands) > 1:
                m = re.match(r"^I(\d+)$", legacy_id)
                if m:
                    target_id = int(m.group(1))
                    filtered = [c for c in cands if c["target_id"] == target_id]
                    if len(filtered) == 1:
                        picked = filtered[0]

            if picked is None:
                individual_unmatched.append(
                    {
                        "legacyId": legacy_id,
                        "legacyName": legacy_name,
                        "legacyType": legacy_type,
                        "expectedGroupCode": expected_group,
                        "candidateCount": len(cands),
                    }
                )
                continue

            belongs_to = str(row.get("belongs_to") or "")
            expected_parent = mapped_collectives.get(belongs_to)
            if expected_parent is not None and picked.get("parent_object_id") != expected_parent.get("id"):
                individual_parent_mismatch.append(
                    {
                        "legacyId": legacy_id,
                        "legacyName": legacy_name,
                        "legacyBelongsTo": belongs_to,
                        "sessionObjectId": picked["id"],
                        "sessionParentObjectId": picked.get("parent_object_id"),
                        "expectedParentObjectId": expected_parent.get("id"),
                    }
                )

            if expected_group and picked.get("group_code") != expected_group:
                individual_group_mismatch.append(
                    {
                        "legacyId": legacy_id,
                        "legacyName": legacy_name,
                        "legacyType": legacy_type,
                        "sessionGroupCode": picked.get("group_code"),
                        "expectedGroupCode": expected_group,
                    }
                )

            mapped_individuals.append(
                {
                    "legacyId": legacy_id,
                    "legacyName": legacy_name,
                    "legacyType": legacy_type,
                    "legacyBelongsTo": belongs_to,
                    "sessionObjectId": picked["id"],
                    "sessionGroupCode": picked["group_code"],
                    "sessionTargetType": picked["target_type"],
                    "sessionTargetId": picked["target_id"],
                    "sessionParentObjectId": picked.get("parent_object_id"),
                }
            )

        existing_rows = conn.execute(
            "SELECT assessment_id, period_code, object_id, module_key, score, detail_json FROM assessment_object_module_scores"
        ).fetchall()
        existing_map: dict[tuple[int, str, int, str], tuple[float, str]] = {}
        for r in existing_rows:
            key = (int(r[0]), str(r[1] or "").strip().upper(), int(r[2]), str(r[3] or "").strip())
            existing_map[key] = (as_float(r[4], 0.0), str(r[5] or ""))

        individual_map = {str(x["legacyId"]): x for x in mapped_individuals}
        candidates_by_key: dict[tuple[int, str, int, str], dict[str, Any]] = {}
        skipped: list[dict[str, Any]] = []

        def put_candidate(candidate: dict[str, Any]) -> None:
            key = (
                int(candidate["assessmentId"]),
                str(candidate["periodCode"]),
                int(candidate["objectId"]),
                str(candidate["moduleKey"]),
            )
            prev = candidates_by_key.get(key)
            if prev is None:
                candidates_by_key[key] = candidate
                return
            # raw_data_vote priority > modules
            rank = {"raw_data_vote": 3, "modules": 2}
            if rank.get(str(candidate.get("sourceKind")), 0) >= rank.get(str(prev.get("sourceKind")), 0):
                candidates_by_key[key] = candidate

        def remap_module_key(
            *,
            entity: dict[str, Any],
            target_period: str,
            target_group: str,
            source_kind: str,
            module_key: str,
        ) -> str:
            legacy_type = str(entity.get("type") or "").strip()
            key = str(module_key or "").strip()
            # Legacy "集团其他高管" annual vote module key was new_module_1.
            # In current session it is merged into general_staff and uses new_module_2.
            if (
                source_kind == "raw_data"
                and legacy_type == "集团其他高管"
                and target_period == "ANNAUL"
                and target_group == "general_staff"
                and key == "new_module_1"
            ):
                return "new_module_2"
            return key

        def process_entity(entity: dict[str, Any], mapped_obj: dict[str, Any]) -> None:
            group_code = str(mapped_obj.get("sessionGroupCode") or mapped_obj.get("group_code") or "")
            object_id = int(mapped_obj.get("sessionObjectId") or mapped_obj.get("id") or 0)
            if not group_code or object_id <= 0:
                return
            scores = entity.get("scores")
            if not isinstance(scores, dict):
                return
            for legacy_period, pdata in scores.items():
                target_period = PERIOD_MAP.get(str(legacy_period or "").strip().upper())
                if not target_period:
                    skipped.append(
                        {
                            "legacyObjectId": str(entity.get("id") or ""),
                            "legacyObjectName": str(entity.get("name") or ""),
                            "legacyPeriod": str(legacy_period),
                            "groupCode": group_code,
                            "reason": "period_not_supported",
                        }
                    )
                    continue
                allowed = modules_by_scope.get((target_period, group_code), set())
                if not isinstance(pdata, dict):
                    pdata = {}
                modules = pdata.get("modules")
                raw_data = pdata.get("raw_data")
                if not isinstance(modules, dict):
                    modules = {}
                if not isinstance(raw_data, dict):
                    raw_data = {}

                for mk_raw, score_raw in modules.items():
                    mk = str(mk_raw or "").strip()
                    if not mk:
                        continue
                    mk = remap_module_key(
                        entity=entity,
                        target_period=target_period,
                        target_group=group_code,
                        source_kind="modules",
                        module_key=mk,
                    )
                    if mk not in allowed:
                        skipped.append(
                            {
                                "legacyObjectId": str(entity.get("id") or ""),
                                "legacyObjectName": str(entity.get("name") or ""),
                                "legacyPeriod": str(legacy_period),
                                "targetPeriod": target_period,
                                "groupCode": group_code,
                                "moduleKey": mk,
                                "sourceKind": "modules",
                                "reason": "module_not_in_current_scope",
                            }
                        )
                        continue
                    node = node_by_scope_module.get((target_period, group_code, mk), {})
                    method = str(node.get("calculationMethod") or "").strip().lower()
                    if method == "vote":
                        continue
                    put_candidate(
                        {
                            "assessmentId": assessment_id,
                            "periodCode": target_period,
                            "objectId": object_id,
                            "moduleKey": mk,
                            "score": as_float(score_raw, 0.0),
                            "detailJson": "",
                            "sourceKind": "modules",
                            "legacyObjectId": str(entity.get("id") or ""),
                            "legacyObjectName": str(entity.get("name") or ""),
                            "legacyObjectType": str(entity.get("type") or ""),
                        }
                    )

                for mk_raw, raw_vote in raw_data.items():
                    mk = str(mk_raw or "").strip()
                    if not mk:
                        continue
                    mk = remap_module_key(
                        entity=entity,
                        target_period=target_period,
                        target_group=group_code,
                        source_kind="raw_data",
                        module_key=mk,
                    )
                    if mk not in allowed:
                        skipped.append(
                            {
                                "legacyObjectId": str(entity.get("id") or ""),
                                "legacyObjectName": str(entity.get("name") or ""),
                                "legacyPeriod": str(legacy_period),
                                "targetPeriod": target_period,
                                "groupCode": group_code,
                                "moduleKey": mk,
                                "sourceKind": "raw_data",
                                "reason": "module_not_in_current_scope",
                            }
                        )
                        continue
                    node = node_by_scope_module.get((target_period, group_code, mk), {})
                    method = str(node.get("calculationMethod") or "").strip().lower()
                    if method != "vote":
                        skipped.append(
                            {
                                "legacyObjectId": str(entity.get("id") or ""),
                                "legacyObjectName": str(entity.get("name") or ""),
                                "legacyPeriod": str(legacy_period),
                                "targetPeriod": target_period,
                                "groupCode": group_code,
                                "moduleKey": mk,
                                "sourceKind": "raw_data",
                                "reason": "raw_data_for_non_vote_module",
                            }
                        )
                        continue
                    try:
                        vote_input = build_vote_input(raw_vote)
                        vote_config = parse_vote_config(node)
                        vote_score, vote_detail = calc_vote_score(vote_config, vote_input)
                        put_candidate(
                            {
                                "assessmentId": assessment_id,
                                "periodCode": target_period,
                                "objectId": object_id,
                                "moduleKey": mk,
                                "score": vote_score,
                                "detailJson": json.dumps(vote_detail, ensure_ascii=False),
                                "sourceKind": "raw_data_vote",
                                "legacyObjectId": str(entity.get("id") or ""),
                                "legacyObjectName": str(entity.get("name") or ""),
                                "legacyObjectType": str(entity.get("type") or ""),
                            }
                        )
                    except Exception as exc:  # noqa: BLE001
                        skipped.append(
                            {
                                "legacyObjectId": str(entity.get("id") or ""),
                                "legacyObjectName": str(entity.get("name") or ""),
                                "legacyPeriod": str(legacy_period),
                                "targetPeriod": target_period,
                                "groupCode": group_code,
                                "moduleKey": mk,
                                "sourceKind": "raw_data",
                                "reason": f"vote_conversion_failed: {exc}",
                            }
                        )

        for row in collectives:
            mapped = mapped_collectives.get(str(row.get("id") or ""))
            if mapped:
                process_entity(row, mapped)
        for row in individuals:
            mapped = individual_map.get(str(row.get("id") or ""))
            if mapped:
                process_entity(row, mapped)

        candidates = list(candidates_by_key.values())
        candidates.sort(key=lambda x: (x["periodCode"], x["objectId"], x["moduleKey"]))

        action_count = {"insert": 0, "update": 0, "noop": 0}
        for row in candidates:
            key = (int(row["assessmentId"]), str(row["periodCode"]), int(row["objectId"]), str(row["moduleKey"]))
            old = existing_map.get(key)
            if old is None:
                action = "insert"
            else:
                same_score = abs(as_float(old[0], 0.0) - as_float(row["score"], 0.0)) < 1e-9
                same_detail = str(old[1] or "") == str(row.get("detailJson") or "")
                action = "noop" if (same_score and same_detail) else "update"
            row["action"] = action
            action_count[action] += 1

        now_tag = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
        output_dir = (
            Path(args.output_dir).expanduser().resolve()
            if str(args.output_dir or "").strip()
            else session_db.parent / "migration-backups"
        )
        output_dir.mkdir(parents=True, exist_ok=True)

        report_path = output_dir / f"legacy_business_mapping_report_{now_tag}.json"
        candidates_path = output_dir / f"legacy_business_module_score_candidates_{now_tag}.json"

        collective_mappings = []
        for row in collectives:
            legacy_id = str(row.get("id") or "")
            mapped = mapped_collectives.get(legacy_id)
            if mapped is None:
                continue
            collective_mappings.append(
                {
                    "legacyId": legacy_id,
                    "legacyName": str(row.get("name") or ""),
                    "legacyType": str(row.get("type") or ""),
                    "sessionObjectId": mapped["id"],
                    "sessionGroupCode": mapped["group_code"],
                    "sessionTargetType": mapped["target_type"],
                    "sessionTargetId": mapped["target_id"],
                }
            )

        report = {
            "generatedAt": dt.datetime.now().isoformat(timespec="seconds"),
            "legacyDir": str(legacy_dir),
            "sessionDb": str(session_db),
            "assessmentId": assessment_id,
            "ruleId": rule_id,
            "legacyCounts": {"collectives": len(collectives), "individuals": len(individuals)},
            "sessionCounts": {"objects": len(objects), "teams": len(teams), "individuals": len(session_individuals)},
            "mapping": {
                "collectiveMappedCount": len(mapped_collectives),
                "collectiveMappings": collective_mappings,
                "collectiveUnmatched": collective_unmatched,
                "collectiveAmbiguous": collective_ambiguous,
                "individualMappedCount": len(mapped_individuals),
                "individualMappings": mapped_individuals,
                "individualUnmatched": individual_unmatched,
                "individualParentMismatch": individual_parent_mismatch,
                "individualGroupMismatch": individual_group_mismatch,
            },
            "candidates": {
                "total": len(candidates),
                "insert": action_count["insert"],
                "update": action_count["update"],
                "noop": action_count["noop"],
            },
            "skipped": {"count": len(skipped), "items": skipped},
        }

        report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        candidates_path.write_text(json.dumps(candidates, ensure_ascii=False, indent=2), encoding="utf-8")

        print(f"legacy_dir: {legacy_dir}")
        print(f"session_db: {session_db}")
        print(f"assessment_id: {assessment_id}")
        print(f"rule_id: {rule_id}")
        print("---- mapping ----")
        print(f"collective_mapped: {len(mapped_collectives)}")
        print(f"collective_unmatched: {len(collective_unmatched)}")
        print(f"collective_ambiguous: {len(collective_ambiguous)}")
        print(f"individual_mapped: {len(mapped_individuals)}")
        print(f"individual_unmatched: {len(individual_unmatched)}")
        print(f"individual_parent_mismatch: {len(individual_parent_mismatch)}")
        print(f"individual_group_mismatch: {len(individual_group_mismatch)}")
        print("---- candidates ----")
        print(f"total: {len(candidates)}")
        print(f"insert: {action_count['insert']}")
        print(f"update: {action_count['update']}")
        print(f"noop: {action_count['noop']}")
        print("---- skipped ----")
        print(f"count: {len(skipped)}")
        print("---- output ----")
        print(f"report: {report_path}")
        print(f"candidates: {candidates_path}")
        return 0
    finally:
        conn.close()


if __name__ == "__main__":
    raise SystemExit(main())
