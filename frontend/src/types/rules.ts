export interface RuleFileItem {
  id: number;
  assessmentId: number;
  ruleName: string;
  description: string;
  contentJson: string;
  filePath: string;
  isCopy: boolean;
  sourceRuleId?: number;
  ownerOrgId?: number;
  createdAt: number;
  updatedAt: number;
  hiddenByCurrentOrg?: boolean;
  canEdit?: boolean;
  canDelete?: boolean;
}

export interface UpdateRuleFilePayload {
  assessmentId: number;
  ruleName?: string;
  description?: string;
  contentJson?: string;
}

export interface RuleDependencyIssue {
  severity: "error" | "warning";
  code: string;
  message: string;
  path?: string[];
}

export interface RuleDependencyCheckSummary {
  errorCount: number;
  warningCount: number;
  nodeCount: number;
  edgeCount: number;
}

export interface RuleDependencyCheckResult {
  summary: RuleDependencyCheckSummary;
  issues: RuleDependencyIssue[];
}

export interface RuleExpressionVariable {
  name: string;
  type: string;
  description: string;
  insertText: string;
}

export interface RuleExpressionFunction {
  name: string;
  signature: string;
  returnType: string;
  description: string;
  insertText: string;
}

export interface RuleExpressionObjectOption {
  objectId: number;
  objectName: string;
  objectType: string;
  groupCode: string;
  targetType: string;
  targetId: number;
  parentObjectId?: number;
  isPriority?: boolean;
}

export interface RuleExpressionContext {
  assessmentId: number;
  periodCode?: string;
  objectGroupCode?: string;
  moduleVariables: RuleExpressionVariable[];
  gradeVariables: RuleExpressionVariable[];
  functions: RuleExpressionFunction[];
  periods: string[];
  objects: RuleExpressionObjectOption[];
}
