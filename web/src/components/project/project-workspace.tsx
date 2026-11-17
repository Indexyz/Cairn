import type { RefObject } from "react";

import type { ProjectDetail } from "@/lib/types";
import { GraphToolbar, type GraphToolbarProps } from "@/components/project/graph-toolbar";
import { ProjectDialogs, type ProjectDialogsProps } from "@/components/project/project-dialogs";
import { ProjectHeader, type ProjectHeaderProps } from "@/components/project/project-header";
import { ProjectSidePanel, type ProjectSidePanelProps } from "@/components/project/project-side-panel";

export interface ProjectWorkspaceProps {
  project: ProjectDetail;
  header: ProjectHeaderProps;
  toolbar: GraphToolbarProps;
  panel: ProjectSidePanelProps;
  dialogs: ProjectDialogsProps;
  graphRef: RefObject<HTMLDivElement | null>;
  replayVersion: number;
  panelWidth: number;
  onPanelResizeStart: (event: React.PointerEvent<HTMLDivElement>) => void;
}

export function ProjectWorkspace({
  project,
  header,
  toolbar,
  panel,
  dialogs,
  graphRef,
  replayVersion,
  panelWidth,
  onPanelResizeStart,
}: ProjectWorkspaceProps) {
  return (
    <div className="flex h-full flex-col overflow-hidden">
      <ProjectHeader {...header} />
      <div className="flex min-h-0 flex-1">
        <div className="relative min-w-0 flex-1">
          <GraphToolbar {...toolbar} />
          <div key={replayVersion} ref={graphRef} className="h-full w-full" />
        </div>

        <div
          onPointerDown={onPanelResizeStart}
          className="w-1.5 shrink-0 cursor-col-resize bg-border/60 hover:bg-border"
        />

        <ProjectSidePanel {...panel} width={panelWidth} project={project} />
      </div>

      <ProjectDialogs {...dialogs} projectId={project.project.id} />
    </div>
  );
}
