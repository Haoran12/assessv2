import { reactive } from "vue";

export type UnsavedDecision = "save_and_leave" | "discard_changes" | "return_editing";

interface UnsavedDialogRequest {
  title?: string;
  message?: string;
  saveButtonText?: string;
  discardButtonText?: string;
  returnButtonText?: string;
}

interface UnsavedDialogState {
  visible: boolean;
  title: string;
  message: string;
  saveButtonText: string;
  discardButtonText: string;
  returnButtonText: string;
}

const defaultDialogState: UnsavedDialogState = {
  visible: false,
  title: "未保存改动提醒",
  message: "检测到存在未保存改动，请选择下一步操作。",
  saveButtonText: "保存并离开",
  discardButtonText: "放弃改动",
  returnButtonText: "返回编辑",
};

const state = reactive<UnsavedDialogState>({
  ...defaultDialogState,
});

let pendingResolve: ((value: UnsavedDecision) => void) | null = null;
let pendingPromise: Promise<UnsavedDecision> | null = null;

export function useUnsavedDialogState(): UnsavedDialogState {
  return state;
}

export function openUnsavedDialog(request?: UnsavedDialogRequest): Promise<UnsavedDecision> {
  if (pendingPromise) {
    return pendingPromise;
  }

  state.title = request?.title || defaultDialogState.title;
  state.message = request?.message || defaultDialogState.message;
  state.saveButtonText = request?.saveButtonText || defaultDialogState.saveButtonText;
  state.discardButtonText = request?.discardButtonText || defaultDialogState.discardButtonText;
  state.returnButtonText = request?.returnButtonText || defaultDialogState.returnButtonText;
  state.visible = true;

  pendingPromise = new Promise<UnsavedDecision>((resolve) => {
    pendingResolve = resolve;
  });

  return pendingPromise;
}

export function resolveUnsavedDialog(decision: UnsavedDecision): void {
  state.visible = false;

  const resolver = pendingResolve;
  pendingResolve = null;
  pendingPromise = null;
  if (resolver) {
    resolver(decision);
  }
}

export function cancelUnsavedDialog(): void {
  resolveUnsavedDialog("return_editing");
}
