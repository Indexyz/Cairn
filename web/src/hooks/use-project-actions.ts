import type { Dispatch, SetStateAction } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useToast } from "@/hooks/use-toast";
import { api, fetchText } from "@/lib/api";
import type { Intent, ProjectStatus } from "@/lib/types";

interface Options {
  projectId: string;
  actorName: string;
  load: (id: string) => Promise<void>;
  selectedFacts: string[];
  actionableIntent: Intent | null;
  releasableIntent: Intent | null;
  concludeTarget: Intent | null;
  intentText: string;
  concludeText: string;
  completeText: string;
  hintText: string;
  renameText: string;
  reopenText: string;
  maxStepsText: string;
  setIntentOpen: Dispatch<SetStateAction<boolean>>;
  setIntentText: Dispatch<SetStateAction<string>>;
  setConcludeTarget: Dispatch<SetStateAction<Intent | null>>;
  setConcludeText: Dispatch<SetStateAction<string>>;
  setSelectedNode: Dispatch<SetStateAction<import("@/lib/types").Selection | null>>;
  setCompleteOpen: Dispatch<SetStateAction<boolean>>;
  setCompleteText: Dispatch<SetStateAction<string>>;
  setHintOpen: Dispatch<SetStateAction<boolean>>;
  setHintText: Dispatch<SetStateAction<string>>;
  setSideTab: Dispatch<SetStateAction<"detail" | "hints" | "log">>;
  setRenameOpen: Dispatch<SetStateAction<boolean>>;
  setReopenOpen: Dispatch<SetStateAction<boolean>>;
  setReopenText: Dispatch<SetStateAction<string>>;
  setSnapshot: Dispatch<SetStateAction<string | null>>;
  setMaxStepsOpen: Dispatch<SetStateAction<boolean>>;
}

export function useProjectActions(options: Options) {
  const {
    projectId,
    actorName,
    load,
    selectedFacts,
    actionableIntent,
    releasableIntent,
    concludeTarget,
    intentText,
    concludeText,
    completeText,
    hintText,
    renameText,
    reopenText,
    maxStepsText,
    setIntentOpen,
    setIntentText,
    setConcludeTarget,
    setConcludeText,
    setSelectedNode,
    setCompleteOpen,
    setCompleteText,
    setHintOpen,
    setHintText,
    setSideTab,
    setRenameOpen,
    setReopenOpen,
    setReopenText,
    setSnapshot,
    setMaxStepsOpen,
  } = options;
  const { t } = useTranslation();
  const toast = useToast();
  const navigate = useNavigate();

  async function setStatus(next: ProjectStatus) {
    try {
      await api("PUT", `/projects/${projectId}/status`, { status: next });
      await load(projectId);
      toast.show(t(next === "stopped" ? "project.toast.stopped" : "project.toast.resumed"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function saveMaxSteps() {
    try {
      await api("PUT", `/projects/${projectId}/max_steps`, {
        max_steps: maxStepsText.trim() ? Number(maxStepsText) : null,
      });
      setMaxStepsOpen(false);
      await load(projectId);
      toast.show(t("project.toast.maxSteps"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function createIntent(claim: boolean) {
    try {
      await api("POST", `/projects/${projectId}/intents`, {
        from: selectedFacts,
        description: intentText,
        creator: actorName,
        worker: claim ? actorName : null,
      });
      setIntentOpen(false);
      setIntentText("");
      await load(projectId);
      toast.show(t(claim ? "project.toast.intentClaimed" : "project.toast.intentDeclared"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function heartbeatSelected() {
    if (!actionableIntent) return;
    try {
      const claiming = !actionableIntent.worker;
      await api("POST", `/projects/${projectId}/intents/${actionableIntent.id}/heartbeat`, {
        worker: actorName,
      });
      await load(projectId);
      toast.show(t(claiming ? "project.toast.claimed" : "project.toast.heartbeat"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function releaseSelected() {
    if (!releasableIntent) return;
    try {
      await api("POST", `/projects/${projectId}/intents/${releasableIntent.id}/release`, {
        worker: actorName,
      });
      await load(projectId);
      toast.show(t("project.toast.released"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function concludeIntent() {
    if (!concludeTarget) return;
    try {
      await api("POST", `/projects/${projectId}/intents/${concludeTarget.id}/conclude`, {
        worker: actorName,
        description: concludeText,
      });
      setConcludeTarget(null);
      setConcludeText("");
      setSelectedNode(null);
      await load(projectId);
      toast.show(t("project.toast.concluded"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function completeProject() {
    try {
      await api("POST", `/projects/${projectId}/complete`, {
        from: selectedFacts,
        description: completeText,
        worker: actorName,
      });
      setCompleteOpen(false);
      setCompleteText("");
      await load(projectId);
      toast.show(t("project.toast.completed"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function addHint() {
    try {
      await api("POST", `/projects/${projectId}/hints`, { content: hintText, creator: actorName });
      setHintOpen(false);
      setHintText("");
      await load(projectId);
      setSideTab("hints");
      toast.show(t("project.toast.hintAdded"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function renameProject() {
    try {
      await api("PUT", `/projects/${projectId}/title`, { title: renameText });
      setRenameOpen(false);
      await load(projectId);
      toast.show(t("project.toast.renamed"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function reopenProject() {
    try {
      await api("POST", `/projects/${projectId}/reopen`, {
        description: reopenText,
        creator: actorName,
      });
      setReopenOpen(false);
      setReopenText("");
      await load(projectId);
      toast.show(t("project.toast.reopened"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function deleteProject() {
    try {
      await api("DELETE", `/projects/${projectId}`);
      toast.show(t("project.toast.deleted", { id: projectId }));
      navigate("/");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function openSnapshot() {
    try {
      setSnapshot(await fetchText(`/projects/${projectId}/export?format=yaml`));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  return {
    setStatus,
    saveMaxSteps,
    createIntent,
    heartbeatSelected,
    releaseSelected,
    concludeIntent,
    completeProject,
    addHint,
    renameProject,
    reopenProject,
    deleteProject,
    openSnapshot,
  };
}
