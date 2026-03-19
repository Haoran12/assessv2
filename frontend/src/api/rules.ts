import { http } from "@/api/http";
import type {
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
