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

export interface RuleBindingItem {
  id: number;
  assessmentId: number;
  periodCode: string;
  objectGroupCode: string;
  organizationId: number;
  ruleFileId: number;
  createdAt: number;
  updatedAt: number;
  ruleFile: RuleFileItem;
}

export interface CreateRuleFilePayload {
  assessmentId: number;
  ruleName: string;
  description?: string;
  contentJson?: string;
}

export interface UpdateRuleFilePayload {
  assessmentId: number;
  ruleName?: string;
  description?: string;
  contentJson?: string;
}

export interface SelectRuleBindingPayload {
  assessmentId: number;
  periodCode: string;
  objectGroupCode: string;
  sourceRuleId: number;
}
