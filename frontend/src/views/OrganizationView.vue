
<template>
  <div class="org-view">
    <el-row :gutter="16" class="layout-row">
      <el-col :xs="24" :lg="8">
        <el-card class="tree-card" shadow="never">
          <template #header>
            <div class="card-header">
              <strong>组织树</strong>
              <el-button size="small" :loading="loadingTree" @click="loadTree">刷新</el-button>
            </div>
          </template>

          <div class="tree-toolbar">
            <el-input
              v-model="treeKeyword"
              clearable
              placeholder="搜索组织/部门/人员"
            />
            <el-switch
              v-model="includeInactive"
              active-text="包含停用"
              @change="loadTree"
            />
          </div>

          <el-tree
            ref="treeRef"
            class="org-tree"
            :data="treeData"
            node-key="treeKey"
            default-expand-all
            highlight-current
            :props="treeProps"
            :filter-node-method="filterTreeNode"
            @node-click="handleTreeNodeClick"
          >
            <template #default="{ data }">
              <div class="tree-node-row">
                <span class="tree-node-title">{{ data.name }}</span>
                <el-tag size="small" :type="statusTagType(data.status)">
                  {{ statusText(data.status) }}
                </el-tag>
              </div>
            </template>
          </el-tree>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="16">
        <el-tabs v-model="activeTab" type="border-card">
          <el-tab-pane label="组织管理" name="organizations">
            <div class="toolbar-grid">
              <el-input
                v-model="orgQuery.keyword"
                clearable
                placeholder="按组织名称搜索"
                @keyup.enter="loadOrganizations"
                @clear="loadOrganizations"
              />
              <el-select v-model="orgQuery.status" clearable placeholder="状态" @change="loadOrganizations">
                <el-option label="启用" value="active" />
                <el-option label="停用" value="inactive" />
              </el-select>
              <el-button :loading="loadingOrganizations" @click="loadOrganizations">查询</el-button>
              <el-button type="primary" :disabled="!canEdit" @click="openOrganizationDialog()">新增组织</el-button>
            </div>

            <el-table v-loading="loadingOrganizations" :data="organizations" border>
              <el-table-column prop="id" label="ID" width="70" />
              <el-table-column prop="orgName" label="组织名称" min-width="180" />
              <el-table-column prop="orgType" label="类型" width="110">
                <template #default="{ row }">
                  {{ row.orgType === "group" ? "集团" : "公司" }}
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :disabled="!canEdit" @click="openOrganizationDialog(row)">
                    编辑
                  </el-button>
                  <el-button v-if="isRoot" link type="danger" @click="handleDeleteOrganization(row)">
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="部门管理" name="departments">
            <div class="toolbar-grid">
              <el-select
                v-model="deptQuery.organizationId"
                clearable
                filterable
                placeholder="所属组织"
                @change="loadDepartments"
              >
                <el-option
                  v-for="org in organizations"
                  :key="org.id"
                  :label="org.orgName"
                  :value="org.id"
                />
              </el-select>
              <el-input
                v-model="deptQuery.keyword"
                clearable
                placeholder="按部门名称搜索"
                @keyup.enter="loadDepartments"
                @clear="loadDepartments"
              />
              <el-select v-model="deptQuery.status" clearable placeholder="状态" @change="loadDepartments">
                <el-option label="启用" value="active" />
                <el-option label="停用" value="inactive" />
              </el-select>
              <el-button :loading="loadingDepartments" @click="loadDepartments">查询</el-button>
              <el-button type="primary" :disabled="!canEdit" @click="openDepartmentDialog()">新增部门</el-button>
            </div>

            <el-table v-loading="loadingDepartments" :data="departments" border>
              <el-table-column prop="id" label="ID" width="70" />
              <el-table-column prop="deptName" label="部门名称" min-width="180" />
              <el-table-column label="所属组织" min-width="170">
                <template #default="{ row }">
                  {{ organizationName(row.organizationId) }}
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :disabled="!canEdit" @click="openDepartmentDialog(row)">
                    编辑
                  </el-button>
                  <el-button v-if="isRoot" link type="danger" @click="handleDeleteDepartment(row)">
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="分类管理" name="positionLevels">
            <div class="toolbar-grid">
              <el-input
                v-model="positionLevelQuery.keyword"
                clearable
                placeholder="按分类名称/编码搜索"
              />
              <el-select v-model="positionLevelQuery.status" clearable placeholder="状态">
                <el-option label="启用" value="active" />
                <el-option label="停用" value="inactive" />
              </el-select>
              <el-button :loading="loadingPositionLevels" @click="loadPositionLevels">查询</el-button>
              <el-button
                type="primary"
                :disabled="!canManagePositionLevels"
                @click="openPositionLevelDialog()"
              >
                新增分类
              </el-button>
            </div>

            <el-table v-loading="loadingPositionLevels" :data="filteredPositionLevels" border>
              <el-table-column prop="id" label="ID" width="70" />
              <el-table-column prop="levelName" label="分类名称" min-width="160" />
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :disabled="!canManagePositionLevels" @click="openPositionLevelDialog(row)">
                    编辑
                  </el-button>
                  <el-button
                    link
                    type="danger"
                    :disabled="!canManagePositionLevels"
                    @click="handleDeletePositionLevel(row)"
                  >
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="人员管理" name="employees">
            <div class="toolbar-grid toolbar-grid-employee">
              <el-select v-model="employeeQuery.status" clearable placeholder="状态" @change="loadEmployees">
                <el-option label="在岗" value="active" />
                <el-option label="离岗" value="inactive" />
              </el-select>
              <el-input
                v-model="employeeQuery.keyword"
                clearable
                placeholder="按姓名搜索"
                @keyup.enter="loadEmployees"
                @clear="loadEmployees"
              />
              <el-button :loading="loadingEmployees" @click="loadEmployees">查询</el-button>
              <el-button type="primary" :disabled="!canEdit" @click="openEmployeeDialog()">新增人员</el-button>
            </div>

            <el-table v-loading="loadingEmployees" :data="employees" border>
              <el-table-column prop="id" label="ID" width="70" />
              <el-table-column prop="empName" label="姓名" min-width="120" />
              <el-table-column label="所属组织" min-width="150">
                <template #default="{ row }">
                  {{ organizationName(row.organizationId) }}
                </template>
              </el-table-column>
              <el-table-column label="所属部门" min-width="150">
                <template #default="{ row }">
                  {{ row.departmentId ? departmentName(row.departmentId) : "-" }}
                </template>
              </el-table-column>
              <el-table-column label="分类" min-width="130">
                <template #default="{ row }">
                  {{ positionLevelName(row.positionLevelId) }}
                </template>
              </el-table-column>
              <el-table-column prop="positionTitle" label="岗位" min-width="130" />
              <el-table-column label="入职日期" min-width="130">
                <template #default="{ row }">
                  {{ row.hireDate ? dateText(row.hireDate) : "-" }}
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" min-width="280" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :disabled="!canEdit" @click="openEmployeeDialog(row)">
                    编辑
                  </el-button>
                  <el-button link type="warning" :disabled="!canEdit" @click="openTransferDialog(row)">
                    调动
                  </el-button>
                  <el-button v-if="isRoot" link type="danger" @click="handleDeleteEmployee(row)">
                    删除
                  </el-button>
                  <el-button link type="success" @click="openHistoryDialog(row)">
                    历史
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-col>
    </el-row>

    <el-dialog v-model="organizationDialogVisible" width="560px" :title="organizationForm.id ? '编辑组织' : '新增组织'">
      <el-form label-width="100px">
        <el-form-item label="组织名称" required>
          <el-input v-model="organizationForm.orgName" maxlength="200" />
        </el-form-item>
        <el-form-item label="组织类型" required>
          <el-select v-model="organizationForm.orgType" style="width: 100%">
            <el-option label="集团" value="group" />
            <el-option label="公司" value="company" />
          </el-select>
        </el-form-item>
        <el-form-item label="上级组织">
          <el-select v-model="organizationForm.parentId" clearable filterable style="width: 100%">
            <el-option
              v-for="org in organizations.filter((item) => item.id !== organizationForm.id)"
              :key="org.id"
              :label="org.orgName"
              :value="org.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="organizationForm.sortOrder" :min="0" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="organizationForm.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="organizationDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingOrganization" @click="submitOrganization">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="departmentDialogVisible" width="560px" :title="departmentForm.id ? '编辑部门' : '新增部门'">
      <el-form label-width="100px">
        <el-form-item label="部门名称" required>
          <el-input v-model="departmentForm.deptName" maxlength="200" />
        </el-form-item>
        <el-form-item label="所属组织" required>
          <el-select v-model="departmentForm.organizationId" filterable style="width: 100%">
            <el-option
              v-for="org in organizations"
              :key="org.id"
              :label="org.orgName"
              :value="org.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="上级部门">
          <el-select v-model="departmentForm.parentDeptId" clearable filterable style="width: 100%">
            <el-option
              v-for="dept in departmentOptionsByOrg(departmentForm.organizationId).filter((item) => item.id !== departmentForm.id)"
              :key="dept.id"
              :label="dept.deptName"
              :value="dept.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="departmentForm.sortOrder" :min="0" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="departmentForm.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="departmentDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingDepartment" @click="submitDepartment">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="positionLevelDialogVisible"
      width="560px"
      :title="positionLevelForm.id ? '编辑分类' : '新增分类'"
    >
      <el-form label-width="110px">
        <el-form-item label="分类编码" required>
          <el-input
            v-model="positionLevelForm.levelCode"
            maxlength="50"
            :disabled="positionLevelForm.isSystem"
            placeholder="示例：department_main"
          />
        </el-form-item>
        <el-form-item label="分类名称" required>
          <el-input v-model="positionLevelForm.levelName" maxlength="100" />
        </el-form-item>
        <el-form-item label="用于考核对象">
          <el-switch v-model="positionLevelForm.isForAssessment" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="positionLevelForm.sortOrder" :min="0" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="positionLevelForm.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="inactive" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="positionLevelForm.description"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="positionLevelDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingPositionLevel" @click="submitPositionLevel">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="employeeDialogVisible" width="640px" :title="employeeForm.id ? '编辑人员' : '新增人员'">
      <el-form label-width="100px">
        <el-row :gutter="12">
          <el-col :span="24">
            <el-form-item label="姓名" required>
              <el-input v-model="employeeForm.empName" maxlength="100" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="所属组织" required>
              <el-select v-model="employeeForm.organizationId" filterable style="width: 100%" @change="onEmployeeFormOrgChange">
                <el-option
                  v-for="org in organizations"
                  :key="org.id"
                  :label="org.orgName"
                  :value="org.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属部门">
              <el-select v-model="employeeForm.departmentId" clearable filterable style="width: 100%">
                <el-option
                  v-for="dept in departmentOptionsByOrg(employeeForm.organizationId)"
                  :key="dept.id"
                  :label="dept.deptName"
                  :value="dept.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="分类" required>
              <el-select v-model="employeeForm.positionLevelId" filterable style="width: 100%">
                <el-option
                  v-for="level in activePositionLevels"
                  :key="level.id"
                  :label="`${level.levelName} (${level.levelCode})`"
                  :value="level.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="岗位名称">
              <el-input v-model="employeeForm.positionTitle" maxlength="100" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="入职日期">
              <el-date-picker
                v-model="employeeForm.hireDate"
                type="date"
                value-format="YYYY-MM-DD"
                placeholder="选择日期"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-select v-model="employeeForm.status" style="width: 100%">
                <el-option label="在岗" value="active" />
                <el-option label="离岗" value="inactive" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="employeeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingEmployee" @click="submitEmployee">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="transferDialogVisible" width="620px" title="人员调动">
      <el-form label-width="110px">
        <el-form-item label="调动类型" required>
          <el-select v-model="transferForm.changeType" style="width: 100%">
            <el-option label="平级调动" value="transfer" />
            <el-option label="晋升" value="promotion" />
            <el-option label="降级" value="demotion" />
            <el-option label="岗位变更" value="position_change" />
          </el-select>
        </el-form-item>
        <el-form-item label="新组织">
          <el-select v-model="transferForm.newOrganizationId" clearable filterable style="width: 100%" @change="onTransferOrgChange">
            <el-option
              v-for="org in organizations"
              :key="org.id"
              :label="org.orgName"
              :value="org.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="新部门">
          <el-select v-model="transferForm.newDepartmentId" clearable filterable style="width: 100%">
            <el-option
              v-for="dept in departmentOptionsByOrg(transferForm.newOrganizationId)"
              :key="dept.id"
              :label="dept.deptName"
              :value="dept.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="新分类">
          <el-select v-model="transferForm.newPositionLevelId" clearable filterable style="width: 100%">
            <el-option
              v-for="level in activePositionLevels"
              :key="level.id"
              :label="`${level.levelName} (${level.levelCode})`"
              :value="level.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="新岗位名称">
          <el-input v-model="transferForm.newPositionTitle" maxlength="100" />
        </el-form-item>
        <el-form-item label="生效日期" required>
          <el-date-picker
            v-model="transferForm.effectiveDate"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="选择生效日期"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="调动原因" required>
          <el-input v-model="transferForm.changeReason" type="textarea" :rows="3" maxlength="200" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="transferDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingTransfer" @click="submitTransfer">确认调动</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="historyDialogVisible" width="860px" :title="`调动历史 - ${historyTargetName}`">
      <el-table :data="historyRows" border>
        <el-table-column prop="changeType" label="调动类型" width="120" />
        <el-table-column label="组织变化" min-width="220">
          <template #default="{ row }">
            {{ organizationName(row.oldOrganizationId) }} -> {{ organizationName(row.newOrganizationId) }}
          </template>
        </el-table-column>
        <el-table-column label="部门变化" min-width="220">
          <template #default="{ row }">
            {{ departmentName(row.oldDepartmentId) }} -> {{ departmentName(row.newDepartmentId) }}
          </template>
        </el-table-column>
        <el-table-column label="分类变化" min-width="220">
          <template #default="{ row }">
            {{ positionLevelName(row.oldPositionLevelId) }} -> {{ positionLevelName(row.newPositionLevelId) }}
          </template>
        </el-table-column>
        <el-table-column prop="effectiveDate" label="生效日期" width="120">
          <template #default="{ row }">{{ dateText(row.effectiveDate) }}</template>
        </el-table-column>
        <el-table-column prop="changeReason" label="原因" min-width="180" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useUnsavedStore } from "@/stores/unsaved";
import {
  createPositionLevel,
  createDepartment,
  createEmployee,
  createOrganization,
  deleteDepartment,
  deleteEmployee,
  deleteOrganization,
  deletePositionLevel,
  getOrgTree,
  listDepartments,
  listEmployeeHistory,
  listEmployees,
  listOrganizations,
  listPositionLevels,
  transferEmployee,
  updatePositionLevel,
  updateDepartment,
  updateEmployee,
  updateOrganization,
} from "@/api/org";
import type {
  DepartmentItem,
  EmployeeHistoryItem,
  EmployeeItem,
  OrgStatus,
  OrgTreeNode,
  OrganizationItem,
  PositionLevelItem,
  TransferType,
  UpsertPositionLevelPayload,
} from "@/types/org";

interface TreeNodeUI extends OrgTreeNode {
  treeKey: string;
  children?: TreeNodeUI[];
}

const treeRef = ref();
const appStore = useAppStore();
const unsavedStore = useUnsavedStore();
const canEdit = computed(() => appStore.hasPermission("org:update"));
const isRoot = computed(() => appStore.primaryRole === "root" || appStore.roles.includes("root"));
const canManagePositionLevels = computed(() => isRoot.value);

const organizationDirtySourceId = "org:organization-dialog";
const departmentDirtySourceId = "org:department-dialog";
const positionLevelDirtySourceId = "org:position-level-dialog";
const employeeDirtySourceId = "org:employee-dialog";
const transferDirtySourceId = "org:transfer-dialog";

const activeTab = ref("organizations");

const treeKeyword = ref("");
const includeInactive = ref(false);
const loadingTree = ref(false);
const treeData = ref<TreeNodeUI[]>([]);

const loadingOrganizations = ref(false);
const organizations = ref<OrganizationItem[]>([]);
const orgQuery = reactive({
  keyword: "",
  status: "" as OrgStatus | "",
});

const loadingDepartments = ref(false);
const departments = ref<DepartmentItem[]>([]);
const deptQuery = reactive({
  organizationId: undefined as number | undefined,
  keyword: "",
  status: "" as OrgStatus | "",
});

const loadingEmployees = ref(false);
const employees = ref<EmployeeItem[]>([]);
const employeeQuery = reactive({
  organizationId: undefined as number | undefined,
  departmentId: undefined as number | undefined,
  keyword: "",
  status: "" as OrgStatus | "",
});

const loadingPositionLevels = ref(false);
const positionLevels = ref<PositionLevelItem[]>([]);
const positionLevelQuery = reactive({
  keyword: "",
  status: "" as OrgStatus | "",
});

const positionLevelDialogVisible = ref(false);
const savingPositionLevel = ref(false);
const positionLevelForm = reactive({
  id: null as number | null,
  levelCode: "",
  levelName: "",
  description: "",
  isForAssessment: true,
  sortOrder: 0,
  status: "active" as OrgStatus,
  isSystem: false,
});

const organizationDialogVisible = ref(false);
const savingOrganization = ref(false);
const organizationForm = reactive({
  id: null as number | null,
  orgName: "",
  orgType: "company" as "group" | "company",
  parentId: undefined as number | undefined,
  sortOrder: 0,
  status: "active" as OrgStatus,
});
const organizationBaseline = ref("");

const departmentDialogVisible = ref(false);
const savingDepartment = ref(false);
const departmentForm = reactive({
  id: null as number | null,
  deptName: "",
  organizationId: undefined as number | undefined,
  parentDeptId: undefined as number | undefined,
  sortOrder: 0,
  status: "active" as OrgStatus,
});
const departmentBaseline = ref("");

const employeeDialogVisible = ref(false);
const savingEmployee = ref(false);
const employeeForm = reactive({
  id: null as number | null,
  empName: "",
  organizationId: undefined as number | undefined,
  departmentId: undefined as number | undefined,
  positionLevelId: undefined as number | undefined,
  positionTitle: "",
  hireDate: "",
  status: "active" as OrgStatus,
});
const employeeBaseline = ref("");

const transferDialogVisible = ref(false);
const savingTransfer = ref(false);
const transferTargetEmployee = ref<EmployeeItem | null>(null);
const transferForm = reactive({
  changeType: "transfer" as TransferType,
  newOrganizationId: undefined as number | undefined,
  newDepartmentId: undefined as number | undefined,
  newPositionLevelId: undefined as number | undefined,
  newPositionTitle: "",
  changeReason: "",
  effectiveDate: "",
});
const transferBaseline = ref("");

const historyDialogVisible = ref(false);
const historyTargetName = ref("");
const historyRows = ref<EmployeeHistoryItem[]>([]);
const positionLevelBaseline = ref("");

const selectedTreeNode = ref<TreeNodeUI | null>(null);
const treeProps = {
  label: "name",
  children: "children",
};

watch(treeKeyword, (value) => {
  treeRef.value?.filter(value);
});

watch(activeTab, (value) => {
  if (value === "departments") {
    void loadDepartments();
  }
  if (value === "employees") {
    void loadEmployees();
  }
});

watch(organizationDialogVisible, (visible) => {
  if (visible) {
    resetOrganizationBaseline();
    return;
  }
  organizationBaseline.value = "";
  unsavedStore.clearDirty(organizationDirtySourceId);
});

watch(
  organizationForm,
  () => {
    if (!organizationDialogVisible.value) {
      unsavedStore.clearDirty(organizationDirtySourceId);
      return;
    }
    const current = organizationFormSignature();
    if (!organizationBaseline.value || current === organizationBaseline.value) {
      unsavedStore.clearDirty(organizationDirtySourceId);
      return;
    }
    unsavedStore.markDirty(organizationDirtySourceId);
  },
  { deep: true },
);

watch(departmentDialogVisible, (visible) => {
  if (visible) {
    resetDepartmentBaseline();
    return;
  }
  departmentBaseline.value = "";
  unsavedStore.clearDirty(departmentDirtySourceId);
});

watch(
  departmentForm,
  () => {
    if (!departmentDialogVisible.value) {
      unsavedStore.clearDirty(departmentDirtySourceId);
      return;
    }
    const current = departmentFormSignature();
    if (!departmentBaseline.value || current === departmentBaseline.value) {
      unsavedStore.clearDirty(departmentDirtySourceId);
      return;
    }
    unsavedStore.markDirty(departmentDirtySourceId);
  },
  { deep: true },
);

watch(positionLevelDialogVisible, (visible) => {
  if (visible) {
    resetPositionLevelBaseline();
    return;
  }
  positionLevelBaseline.value = "";
  unsavedStore.clearDirty(positionLevelDirtySourceId);
});

watch(
  positionLevelForm,
  () => {
    if (!positionLevelDialogVisible.value) {
      unsavedStore.clearDirty(positionLevelDirtySourceId);
      return;
    }
    const current = positionLevelFormSignature();
    if (!positionLevelBaseline.value || current === positionLevelBaseline.value) {
      unsavedStore.clearDirty(positionLevelDirtySourceId);
      return;
    }
    unsavedStore.markDirty(positionLevelDirtySourceId);
  },
  { deep: true },
);

watch(employeeDialogVisible, (visible) => {
  if (visible) {
    resetEmployeeBaseline();
    return;
  }
  employeeBaseline.value = "";
  unsavedStore.clearDirty(employeeDirtySourceId);
});

watch(
  employeeForm,
  () => {
    if (!employeeDialogVisible.value) {
      unsavedStore.clearDirty(employeeDirtySourceId);
      return;
    }
    const current = employeeFormSignature();
    if (!employeeBaseline.value || current === employeeBaseline.value) {
      unsavedStore.clearDirty(employeeDirtySourceId);
      return;
    }
    unsavedStore.markDirty(employeeDirtySourceId);
  },
  { deep: true },
);

watch(transferDialogVisible, (visible) => {
  if (visible) {
    resetTransferBaseline();
    return;
  }
  transferBaseline.value = "";
  unsavedStore.clearDirty(transferDirtySourceId);
});

watch(transferTargetEmployee, () => {
  if (transferDialogVisible.value) {
    resetTransferBaseline();
  }
});

watch(
  transferForm,
  () => {
    if (!transferDialogVisible.value) {
      unsavedStore.clearDirty(transferDirtySourceId);
      return;
    }
    const current = transferFormSignature();
    if (!transferBaseline.value || current === transferBaseline.value) {
      unsavedStore.clearDirty(transferDirtySourceId);
      return;
    }
    unsavedStore.markDirty(transferDirtySourceId);
  },
  { deep: true },
);

function filterTreeNode(value: string, data: TreeNodeUI): boolean {
  const text = value.trim().toLowerCase();
  if (!text) {
    return true;
  }
  return data.name.toLowerCase().includes(text);
}

function normalizeTree(nodes: OrgTreeNode[]): TreeNodeUI[] {
  return nodes.map((node) => ({
    ...node,
    treeKey: `${node.nodeType}-${node.id}`,
    children: node.children ? normalizeTree(node.children) : [],
  }));
}

function statusTagType(status?: string): "success" | "warning" | "info" | "danger" {
  switch (status) {
    case "active":
      return "success";
    case "inactive":
      return "info";
    default:
      return "warning";
  }
}

function statusText(status?: string): string {
  if (!status) {
    return "-";
  }
  switch (status) {
    case "active":
      return "启用";
    case "inactive":
      return "停用";
    default:
      return status;
  }
}

function dateText(value?: string): string {
  if (!value) {
    return "-";
  }
  if (value.includes("T")) {
    return value.slice(0, 10);
  }
  return value;
}

function organizationFormSignature(): string {
  return JSON.stringify({
    id: organizationForm.id,
    orgName: organizationForm.orgName,
    orgType: organizationForm.orgType,
    parentId: organizationForm.parentId,
    sortOrder: organizationForm.sortOrder,
    status: organizationForm.status,
  });
}

function departmentFormSignature(): string {
  return JSON.stringify({
    id: departmentForm.id,
    deptName: departmentForm.deptName,
    organizationId: departmentForm.organizationId,
    parentDeptId: departmentForm.parentDeptId,
    sortOrder: departmentForm.sortOrder,
    status: departmentForm.status,
  });
}

function positionLevelFormSignature(): string {
  return JSON.stringify({
    id: positionLevelForm.id,
    levelCode: positionLevelForm.levelCode,
    levelName: positionLevelForm.levelName,
    description: positionLevelForm.description,
    isForAssessment: positionLevelForm.isForAssessment,
    sortOrder: positionLevelForm.sortOrder,
    status: positionLevelForm.status,
    isSystem: positionLevelForm.isSystem,
  });
}

function employeeFormSignature(): string {
  return JSON.stringify({
    id: employeeForm.id,
    empName: employeeForm.empName,
    organizationId: employeeForm.organizationId,
    departmentId: employeeForm.departmentId,
    positionLevelId: employeeForm.positionLevelId,
    positionTitle: employeeForm.positionTitle,
    hireDate: employeeForm.hireDate,
    status: employeeForm.status,
  });
}

function transferFormSignature(): string {
  return JSON.stringify({
    employeeId: transferTargetEmployee.value?.id ?? null,
    changeType: transferForm.changeType,
    newOrganizationId: transferForm.newOrganizationId,
    newDepartmentId: transferForm.newDepartmentId,
    newPositionLevelId: transferForm.newPositionLevelId,
    newPositionTitle: transferForm.newPositionTitle,
    changeReason: transferForm.changeReason,
    effectiveDate: transferForm.effectiveDate,
  });
}

function resetOrganizationBaseline(): void {
  organizationBaseline.value = organizationFormSignature();
  unsavedStore.clearDirty(organizationDirtySourceId);
}

function resetDepartmentBaseline(): void {
  departmentBaseline.value = departmentFormSignature();
  unsavedStore.clearDirty(departmentDirtySourceId);
}

function resetPositionLevelBaseline(): void {
  positionLevelBaseline.value = positionLevelFormSignature();
  unsavedStore.clearDirty(positionLevelDirtySourceId);
}

function resetEmployeeBaseline(): void {
  employeeBaseline.value = employeeFormSignature();
  unsavedStore.clearDirty(employeeDirtySourceId);
}

function resetTransferBaseline(): void {
  transferBaseline.value = transferFormSignature();
  unsavedStore.clearDirty(transferDirtySourceId);
}

function organizationName(organizationId?: number): string {
  if (!organizationId) {
    return "-";
  }
  return organizations.value.find((item) => item.id === organizationId)?.orgName ?? `#${organizationId}`;
}

function departmentName(departmentId?: number): string {
  if (!departmentId) {
    return "-";
  }
  return departments.value.find((item) => item.id === departmentId)?.deptName ?? `#${departmentId}`;
}

function positionLevelName(levelId?: number): string {
  if (!levelId) {
    return "-";
  }
  return positionLevels.value.find((item) => item.id === levelId)?.levelName ?? `#${levelId}`;
}

const filteredPositionLevels = computed(() => {
  const keyword = positionLevelQuery.keyword.trim().toLowerCase();
  const status = positionLevelQuery.status;
  return positionLevels.value.filter((item) => {
    if (status && item.status !== status) {
      return false;
    }
    if (!keyword) {
      return true;
    }
    const target = `${item.levelName} ${item.levelCode}`.toLowerCase();
    return target.includes(keyword);
  });
});

const activePositionLevels = computed(() => positionLevels.value.filter((item) => item.status === "active"));

function departmentOptionsByOrg(organizationId?: number): DepartmentItem[] {
  if (!organizationId) {
    return departments.value;
  }
  return departments.value.filter((item) => item.organizationId === organizationId);
}

function handleTreeNodeClick(node: TreeNodeUI): void {
  selectedTreeNode.value = node;

  if (node.nodeType === "organization") {
    deptQuery.organizationId = node.id;
    employeeQuery.organizationId = node.id;
    employeeQuery.departmentId = undefined;
  }

  if (node.nodeType === "department") {
    if (node.organizationId) {
      deptQuery.organizationId = node.organizationId;
      employeeQuery.organizationId = node.organizationId;
    }
    employeeQuery.departmentId = node.id;
  }

  if (node.nodeType === "employee") {
    if (node.organizationId) {
      employeeQuery.organizationId = node.organizationId;
    }
    if (node.departmentId) {
      employeeQuery.departmentId = node.departmentId;
    }
  }

  if (activeTab.value === "departments") {
    void loadDepartments();
  }
  if (activeTab.value === "employees") {
    void loadEmployees();
  }
}

async function loadTree(): Promise<void> {
  loadingTree.value = true;
  try {
    const rows = await getOrgTree(includeInactive.value);
    treeData.value = normalizeTree(rows);
  } catch (_error) {
    ElMessage.error("组织树加载失败");
  } finally {
    loadingTree.value = false;
  }
}

async function loadOrganizations(): Promise<void> {
  loadingOrganizations.value = true;
  try {
    organizations.value = await listOrganizations({
      keyword: orgQuery.keyword || undefined,
      status: orgQuery.status || undefined,
    });
  } catch (_error) {
    ElMessage.error("组织列表加载失败");
  } finally {
    loadingOrganizations.value = false;
  }
}

async function loadDepartments(): Promise<void> {
  loadingDepartments.value = true;
  try {
    departments.value = await listDepartments({
      organizationId: deptQuery.organizationId,
      keyword: deptQuery.keyword || undefined,
      status: deptQuery.status || undefined,
    });
  } catch (_error) {
    ElMessage.error("部门列表加载失败");
  } finally {
    loadingDepartments.value = false;
  }
}

async function loadEmployees(): Promise<void> {
  loadingEmployees.value = true;
  try {
    employees.value = await listEmployees({
      organizationId: employeeQuery.organizationId,
      departmentId: employeeQuery.departmentId,
      keyword: employeeQuery.keyword || undefined,
      status: employeeQuery.status || undefined,
    });
  } catch (_error) {
    ElMessage.error("人员列表加载失败");
  } finally {
    loadingEmployees.value = false;
  }
}

async function loadPositionLevels(): Promise<void> {
  loadingPositionLevels.value = true;
  try {
    positionLevels.value = await listPositionLevels();
  } catch (_error) {
    ElMessage.error("分类列表加载失败");
  } finally {
    loadingPositionLevels.value = false;
  }
}

function openOrganizationDialog(item?: OrganizationItem): void {
  if (!canEdit.value) {
    return;
  }
  if (item) {
    Object.assign(organizationForm, {
      id: item.id,
      orgName: item.orgName,
      orgType: item.orgType,
      parentId: item.parentId,
      sortOrder: item.sortOrder,
      status: item.status,
    });
  } else {
    Object.assign(organizationForm, {
      id: null,
      orgName: "",
      orgType: "company",
      parentId: selectedTreeNode.value?.nodeType === "organization" ? selectedTreeNode.value.id : undefined,
      sortOrder: 0,
      status: "active",
    });
  }
  organizationDialogVisible.value = true;
}

async function submitOrganization(): Promise<void> {
  if (!organizationForm.orgName.trim()) {
    ElMessage.warning("请填写组织名称");
    return;
  }

  savingOrganization.value = true;
  try {
    const payload = {
      orgName: organizationForm.orgName.trim(),
      orgType: organizationForm.orgType,
      parentId: organizationForm.parentId,
      sortOrder: organizationForm.sortOrder,
      status: organizationForm.status,
    };
    if (organizationForm.id) {
      await updateOrganization(organizationForm.id, payload);
    } else {
      await createOrganization(payload);
    }
    ElMessage.success("组织已保存");
    organizationDialogVisible.value = false;
    await Promise.all([loadOrganizations(), loadTree()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "组织保存失败";
    ElMessage.error(message);
  } finally {
    savingOrganization.value = false;
  }
}

async function handleDeleteOrganization(item: OrganizationItem): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除组织「${item.orgName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    await deleteOrganization(item.id);
    ElMessage.success("组织已删除");
    await Promise.all([loadOrganizations(), loadDepartments(), loadEmployees(), loadTree()]);
  } catch (error) {
    if (
      error === "cancel" ||
      error === "close" ||
      (error instanceof Error && (error.message === "cancel" || error.message === "close"))
    ) {
      return;
    }
    const message = error instanceof Error ? error.message : "组织删除失败";
    ElMessage.error(message);
  }
}

function openDepartmentDialog(item?: DepartmentItem): void {
  if (!canEdit.value) {
    return;
  }
  if (item) {
    Object.assign(departmentForm, {
      id: item.id,
      deptName: item.deptName,
      organizationId: item.organizationId,
      parentDeptId: item.parentDeptId,
      sortOrder: item.sortOrder,
      status: item.status,
    });
  } else {
    const defaultOrgId =
      selectedTreeNode.value?.nodeType === "organization"
        ? selectedTreeNode.value.id
        : selectedTreeNode.value?.organizationId;
    Object.assign(departmentForm, {
      id: null,
      deptName: "",
      organizationId: defaultOrgId,
      parentDeptId: selectedTreeNode.value?.nodeType === "department" ? selectedTreeNode.value.id : undefined,
      sortOrder: 0,
      status: "active",
    });
  }
  departmentDialogVisible.value = true;
}

async function submitDepartment(): Promise<void> {
  if (!departmentForm.deptName.trim() || !departmentForm.organizationId) {
    ElMessage.warning("请填写部门名称和所属组织");
    return;
  }

  savingDepartment.value = true;
  try {
    const payload = {
      deptName: departmentForm.deptName.trim(),
      organizationId: departmentForm.organizationId,
      parentDeptId: departmentForm.parentDeptId,
      sortOrder: departmentForm.sortOrder,
      status: departmentForm.status,
    };
    if (departmentForm.id) {
      await updateDepartment(departmentForm.id, payload);
    } else {
      await createDepartment(payload);
    }
    ElMessage.success("部门已保存");
    departmentDialogVisible.value = false;
    await Promise.all([loadDepartments(), loadTree()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "部门保存失败";
    ElMessage.error(message);
  } finally {
    savingDepartment.value = false;
  }
}

async function handleDeleteDepartment(item: DepartmentItem): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除部门「${item.deptName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    await deleteDepartment(item.id);
    ElMessage.success("部门已删除");
    await Promise.all([loadDepartments(), loadEmployees(), loadTree()]);
  } catch (error) {
    if (
      error === "cancel" ||
      error === "close" ||
      (error instanceof Error && (error.message === "cancel" || error.message === "close"))
    ) {
      return;
    }
    const message = error instanceof Error ? error.message : "部门删除失败";
    ElMessage.error(message);
  }
}

function openPositionLevelDialog(item?: PositionLevelItem): void {
  if (!canManagePositionLevels.value) {
    return;
  }
  if (item) {
    Object.assign(positionLevelForm, {
      id: item.id,
      levelCode: item.levelCode,
      levelName: item.levelName,
      description: item.description || "",
      isForAssessment: item.isForAssessment,
      sortOrder: item.sortOrder,
      status: item.status,
      isSystem: item.isSystem,
    });
  } else {
    Object.assign(positionLevelForm, {
      id: null,
      levelCode: "",
      levelName: "",
      description: "",
      isForAssessment: true,
      sortOrder: 0,
      status: "active",
      isSystem: false,
    });
  }
  positionLevelDialogVisible.value = true;
}

async function submitPositionLevel(): Promise<void> {
  if (!canManagePositionLevels.value) {
    return;
  }
  if (!positionLevelForm.levelCode.trim() || !positionLevelForm.levelName.trim()) {
    ElMessage.warning("请填写分类编码和名称");
    return;
  }

  const payload: UpsertPositionLevelPayload = {
    levelCode: positionLevelForm.levelCode.trim(),
    levelName: positionLevelForm.levelName.trim(),
    description: positionLevelForm.description.trim() || undefined,
    isForAssessment: positionLevelForm.isForAssessment,
    sortOrder: positionLevelForm.sortOrder,
    status: positionLevelForm.status,
  };

  savingPositionLevel.value = true;
  try {
    if (positionLevelForm.id) {
      await updatePositionLevel(positionLevelForm.id, payload);
    } else {
      await createPositionLevel(payload);
    }
    ElMessage.success("分类已保存");
    positionLevelDialogVisible.value = false;
    await loadPositionLevels();
  } catch (error) {
    const message = error instanceof Error ? error.message : "分类保存失败";
    ElMessage.error(message);
  } finally {
    savingPositionLevel.value = false;
  }
}

async function handleDeletePositionLevel(item: PositionLevelItem): Promise<void> {
  if (!canManagePositionLevels.value) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除分类「${item.levelName} (${item.levelCode})」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    await deletePositionLevel(item.id);
    ElMessage.success("分类已删除");
    await loadPositionLevels();
  } catch (error) {
    if (
      error === "cancel" ||
      error === "close" ||
      (error instanceof Error && (error.message === "cancel" || error.message === "close"))
    ) {
      return;
    }
    const message = error instanceof Error ? error.message : "分类删除失败";
    ElMessage.error(message);
  }
}

function openEmployeeDialog(item?: EmployeeItem): void {
  if (!canEdit.value) {
    return;
  }
  if (item) {
    Object.assign(employeeForm, {
      id: item.id,
      empName: item.empName,
      organizationId: item.organizationId,
      departmentId: item.departmentId,
      positionLevelId: item.positionLevelId,
      positionTitle: item.positionTitle,
      hireDate: dateText(item.hireDate),
      status: item.status,
    });
  } else {
    Object.assign(employeeForm, {
      id: null,
      empName: "",
      organizationId: selectedTreeNode.value?.organizationId,
      departmentId: selectedTreeNode.value?.nodeType === "department" ? selectedTreeNode.value.id : undefined,
      positionLevelId: activePositionLevels.value[0]?.id,
      positionTitle: "",
      hireDate: "",
      status: "active",
    });
  }
  employeeDialogVisible.value = true;
}

function onEmployeeFormOrgChange(): void {
  if (!employeeForm.organizationId) {
    employeeForm.departmentId = undefined;
    return;
  }
  const belongs = departmentOptionsByOrg(employeeForm.organizationId).some(
    (item) => item.id === employeeForm.departmentId,
  );
  if (!belongs) {
    employeeForm.departmentId = undefined;
  }
}

async function submitEmployee(): Promise<void> {
  if (!employeeForm.empName.trim()) {
    ElMessage.warning("请填写姓名");
    return;
  }
  if (!employeeForm.organizationId || !employeeForm.positionLevelId) {
    ElMessage.warning("请选择所属组织和分类");
    return;
  }

  savingEmployee.value = true;
  try {
    const payload = {
      empName: employeeForm.empName.trim(),
      organizationId: employeeForm.organizationId,
      departmentId: employeeForm.departmentId,
      positionLevelId: employeeForm.positionLevelId,
      positionTitle: employeeForm.positionTitle.trim(),
      hireDate: employeeForm.hireDate || undefined,
      status: employeeForm.status,
    };
    if (employeeForm.id) {
      await updateEmployee(employeeForm.id, payload);
    } else {
      await createEmployee(payload);
    }
    ElMessage.success("人员已保存");
    employeeDialogVisible.value = false;
    await Promise.all([loadEmployees(), loadTree()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "人员保存失败";
    ElMessage.error(message);
  } finally {
    savingEmployee.value = false;
  }
}

async function handleDeleteEmployee(item: EmployeeItem): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除人员「${item.empName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    await deleteEmployee(item.id);
    ElMessage.success("人员已删除");
    await Promise.all([loadEmployees(), loadTree()]);
  } catch (error) {
    if (
      error === "cancel" ||
      error === "close" ||
      (error instanceof Error && (error.message === "cancel" || error.message === "close"))
    ) {
      return;
    }
    const message = error instanceof Error ? error.message : "人员删除失败";
    ElMessage.error(message);
  }
}

function openTransferDialog(employee: EmployeeItem): void {
  if (!canEdit.value) {
    return;
  }
  transferTargetEmployee.value = employee;
  Object.assign(transferForm, {
    changeType: "transfer",
    newOrganizationId: employee.organizationId,
    newDepartmentId: employee.departmentId,
    newPositionLevelId: employee.positionLevelId,
    newPositionTitle: employee.positionTitle,
    changeReason: "",
    effectiveDate: new Date().toISOString().slice(0, 10),
  });
  transferDialogVisible.value = true;
}

function onTransferOrgChange(): void {
  if (!transferForm.newOrganizationId) {
    transferForm.newDepartmentId = undefined;
    return;
  }
  const belongs = departmentOptionsByOrg(transferForm.newOrganizationId).some(
    (item) => item.id === transferForm.newDepartmentId,
  );
  if (!belongs) {
    transferForm.newDepartmentId = undefined;
  }
}

async function submitTransfer(): Promise<void> {
  if (!transferTargetEmployee.value) {
    return;
  }
  if (!transferForm.effectiveDate || !transferForm.changeReason.trim()) {
    ElMessage.warning("请填写生效日期和调动原因");
    return;
  }

  savingTransfer.value = true;
  try {
    await transferEmployee(transferTargetEmployee.value.id, {
      changeType: transferForm.changeType,
      newOrganizationId: transferForm.newOrganizationId,
      newDepartmentId: transferForm.newDepartmentId,
      newPositionLevelId: transferForm.newPositionLevelId,
      newPositionTitle: transferForm.newPositionTitle.trim() || undefined,
      changeReason: transferForm.changeReason.trim(),
      effectiveDate: transferForm.effectiveDate,
    });
    ElMessage.success("人员调动已保存");
    transferDialogVisible.value = false;
    await Promise.all([loadEmployees(), loadTree()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "人员调动失败";
    ElMessage.error(message);
  } finally {
    savingTransfer.value = false;
  }
}

async function openHistoryDialog(employee: EmployeeItem): Promise<void> {
  try {
    historyRows.value = await listEmployeeHistory(employee.id);
    historyTargetName.value = employee.empName;
    historyDialogVisible.value = true;
  } catch (_error) {
    ElMessage.error("调动历史加载失败");
  }
}

onMounted(async () => {
  unsavedStore.setSourceMeta(organizationDirtySourceId, {
    label: "组织编辑",
    save: submitOrganization,
  });
  unsavedStore.setSourceMeta(departmentDirtySourceId, {
    label: "部门编辑",
    save: submitDepartment,
  });
  unsavedStore.setSourceMeta(positionLevelDirtySourceId, {
    label: "分类编辑",
    save: submitPositionLevel,
  });
  unsavedStore.setSourceMeta(employeeDirtySourceId, {
    label: "人员编辑",
    save: submitEmployee,
  });
  unsavedStore.setSourceMeta(transferDirtySourceId, {
    label: "人员调动",
    save: submitTransfer,
  });

  await Promise.all([
    loadOrganizations(),
    loadDepartments(),
    loadEmployees(),
    loadPositionLevels(),
    loadTree(),
  ]);
  await nextTick();
  treeRef.value?.filter(treeKeyword.value);
});

onBeforeUnmount(() => {
  unsavedStore.unregisterSource(organizationDirtySourceId);
  unsavedStore.unregisterSource(departmentDirtySourceId);
  unsavedStore.unregisterSource(positionLevelDirtySourceId);
  unsavedStore.unregisterSource(employeeDirtySourceId);
  unsavedStore.unregisterSource(transferDirtySourceId);
});
</script>

<style scoped>
.org-view {
  display: grid;
  gap: 16px;
}

.layout-row {
  width: 100%;
}

.tree-card {
  min-height: 760px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.tree-toolbar {
  display: grid;
  gap: 10px;
  margin-bottom: 12px;
}

.org-tree {
  max-height: 660px;
  overflow: auto;
  padding-right: 8px;
}

.tree-node-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.tree-node-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toolbar-grid {
  display: grid;
  grid-template-columns: 1fr 140px auto auto;
  gap: 12px;
  margin-bottom: 12px;
}

.toolbar-grid-employee {
  grid-template-columns: 120px 1fr auto auto;
}

@media (max-width: 1200px) {
  .tree-card {
    min-height: auto;
  }

  .org-tree {
    max-height: 340px;
  }

  .toolbar-grid,
  .toolbar-grid-employee {
    grid-template-columns: 1fr;
  }
}
</style>
