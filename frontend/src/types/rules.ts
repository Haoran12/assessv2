export type AssessmentPeriodCode = "Q1" | "Q2" | "Q3" | "Q4" | "YEAR_END";
export type RuleObjectType = "team" | "individual";
export type RuleModuleCode = "direct" | "vote" | "custom" | "extra";

export interface RuleSummary {
  id: number;
  yearId: number;
  periodCode: AssessmentPeriodCode;
  objectType: RuleObjectType;
  objectCategory: string;
  ruleName: string;
  description: string;
  isActive: boolean;
  createdAt: number;
  updatedAt: number;
  moduleCount: number;
}

export interface RuleVoteGroup {
  id?: number;
  moduleId?: number;
  groupCode: string;
  groupName: string;
  weight: number;
  voterType: string;
  voterScope?: unknown;
  maxScore: number;
  sortOrder: number;
  isActive: boolean;
}

export interface RuleModule {
  id?: number;
  ruleId?: number;
  moduleCode: RuleModuleCode;
  moduleKey: string;
  moduleName: string;
  weight?: number | null;
  maxScore?: number | null;
  calculationMethod?: string;
  expression?: string;
  contextScope?: unknown;
  sortOrder: number;
  isActive: boolean;
  voteGroups?: RuleVoteGroup[];
}

export interface RuleDetail {
  rule: {
    id: number;
    yearId: number;
    periodCode: AssessmentPeriodCode;
    objectType: RuleObjectType;
    objectCategory: string;
    ruleName: string;
    description: string;
    isActive: boolean;
    createdAt: number;
    updatedAt: number;
  };
  modules: RuleModule[];
}

export interface RuleTemplateConfig {
  ruleName: string;
  description: string;
  modules: RuleModule[];
}

export interface RuleTemplateSummary {
  id: number;
  templateName: string;
  objectType: RuleObjectType;
  objectCategory: string;
  templateConfig: string;
  description: string;
  isSystem: boolean;
  createdAt: number;
  updatedAt: number;
  config: RuleTemplateConfig;
}

export interface CreateRulePayload {
  yearId: number;
  periodCode: AssessmentPeriodCode;
  objectType: RuleObjectType;
  objectCategory: string;
  ruleName: string;
  description: string;
  isActive: boolean;
  syncQuarterly: boolean;
  modules: RuleModule[];
}

export interface UpdateRulePayload {
  ruleName: string;
  description: string;
  isActive: boolean;
  syncQuarterly: boolean;
  modules: RuleModule[];
}

export interface ApplyTemplatePayload {
  yearId: number;
  periodCode: AssessmentPeriodCode;
  objectType: RuleObjectType;
  objectCategory: string;
  ruleName: string;
  description: string;
  syncQuarterly: boolean;
  isActive: boolean;
  overwrite: boolean;
}
