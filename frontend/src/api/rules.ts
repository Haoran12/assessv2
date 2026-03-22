import { http } from "@/api/http";
import type {
  RuleDependencyCheckResult,
  RuleFileItem,
  UpdateRuleFilePayload,
} from "@/types/rules";

export async function listRuleFiles(assessmentId: number, includeHidden = false): Promise<RuleFileItem[]> {
  const response = await http.get("/api/rules/files", {
    params: {
      assessmentId,
      includeHidden,
    },
  });
  return (response.data?.data?.items ?? []) as RuleFileItem[];
}

export async function updateRuleFile(ruleId: number, payload: UpdateRuleFilePayload): Promise<RuleFileItem> {
  const response = await http.put(`/api/rules/files/${ruleId}`, payload);
  return response.data?.data as RuleFileItem;
}

export async function checkRuleDependencies(ruleId: number): Promise<RuleDependencyCheckResult> {
  const response = await http.post(`/api/rules/files/${ruleId}/dependency-check`);
  return response.data?.data as RuleDependencyCheckResult;
}
