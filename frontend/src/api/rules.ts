import { http } from "@/api/http";
import type {
  ApplyTemplatePayload,
  CreateRulePayload,
  RuleModule,
  RuleModuleCode,
  RuleDetail,
  RuleSummary,
  RuleTemplateSummary,
  UpdateRulePayload,
} from "@/types/rules";

interface RulesListParams {
  yearId?: number;
  periodCode?: string;
  objectType?: string;
  objectCategory?: string;
}

interface RuleTemplatesListParams {
  objectType?: string;
  objectCategory?: string;
}

interface RuleModuleFilterParams extends RulesListParams {}

export interface RuleModuleOption {
  id: number;
  ruleId: number;
  ruleName: string;
  moduleCode: RuleModuleCode;
  moduleKey: string;
  moduleName: string;
  maxScore?: number | null;
  objectType: string;
  objectCategory: string;
}

export async function listRules(params: RulesListParams): Promise<RuleSummary[]> {
  const response = await http.get("/api/rules", { params });
  return (response.data?.data?.items ?? []) as RuleSummary[];
}

export async function getRule(ruleId: number): Promise<RuleDetail> {
  const response = await http.get(`/api/rules/${ruleId}`);
  return response.data?.data as RuleDetail;
}

export async function createRule(payload: CreateRulePayload): Promise<RuleDetail> {
  const response = await http.post("/api/rules", payload);
  return response.data?.data as RuleDetail;
}

export async function updateRule(ruleId: number, payload: UpdateRulePayload): Promise<RuleDetail> {
  const response = await http.put(`/api/rules/${ruleId}`, payload);
  return response.data?.data as RuleDetail;
}

export async function listRuleTemplates(params: RuleTemplatesListParams): Promise<RuleTemplateSummary[]> {
  const response = await http.get("/api/rules/templates", { params });
  return (response.data?.data?.items ?? []) as RuleTemplateSummary[];
}

export async function createTemplateFromRule(
  ruleId: number,
  payload: { templateName: string; description: string },
): Promise<RuleTemplateSummary> {
  const response = await http.post(`/api/rules/${ruleId}/templates`, payload);
  return response.data?.data as RuleTemplateSummary;
}

export async function applyRuleTemplate(
  templateId: number,
  payload: ApplyTemplatePayload,
): Promise<RuleDetail> {
  const response = await http.post(`/api/rules/templates/${templateId}/apply`, payload);
  return response.data?.data as RuleDetail;
}

function toModuleOption(rule: RuleSummary, module: RuleModule): RuleModuleOption | null {
  if (!module.id) {
    return null;
  }
  return {
    id: module.id,
    ruleId: rule.id,
    ruleName: rule.ruleName,
    moduleCode: module.moduleCode,
    moduleKey: module.moduleKey,
    moduleName: module.moduleName,
    maxScore: module.maxScore,
    objectType: rule.objectType,
    objectCategory: rule.objectCategory,
  };
}

export async function listRuleModuleOptions(
  params: RuleModuleFilterParams,
  moduleCodes?: RuleModuleCode[],
): Promise<RuleModuleOption[]> {
  const rules = await listRules(params);
  if (rules.length === 0) {
    return [];
  }

  const details = await Promise.all(rules.map((rule) => getRule(rule.id)));
  const codeSet = moduleCodes && moduleCodes.length > 0 ? new Set(moduleCodes) : null;
  const result: RuleModuleOption[] = [];
  for (let index = 0; index < rules.length; index++) {
    const rule = rules[index];
    const modules = details[index].modules || [];
    for (const module of modules) {
      if (!module.isActive) {
        continue;
      }
      if (codeSet && !codeSet.has(module.moduleCode)) {
        continue;
      }
      const option = toModuleOption(rule, module);
      if (option) {
        result.push(option);
      }
    }
  }
  return result;
}
