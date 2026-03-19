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
