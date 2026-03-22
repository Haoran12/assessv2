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
