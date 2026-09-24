"use client";

import { createContext, useContext, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import type { Avatar } from "./account-types";

type SettingsDraft = {
  profileId: string;
  name: string;
  avatar: Avatar;
};

type SettingsStatus = {
  error: boolean;
  message: string;
};

type WorkspacePresentation = {
  friendsQuery: string;
  setFriendsQuery: Dispatch<SetStateAction<string>>;
  friendsOnly: boolean;
  setFriendsOnly: Dispatch<SetStateAction<boolean>>;
  historyBoardId: string | null;
  setHistoryBoardId: Dispatch<SetStateAction<string | null>>;
  settingsDraft: SettingsDraft | null;
  setSettingsDraft: Dispatch<SetStateAction<SettingsDraft | null>>;
  settingsStatus: SettingsStatus;
  setSettingsStatus: Dispatch<SetStateAction<SettingsStatus>>;
};

const WorkspacePresentationContext = createContext<WorkspacePresentation | null>(null);

export function WorkspacePresentationProvider({ children }: { children: ReactNode }) {
  const [friendsQuery, setFriendsQuery] = useState("");
  const [friendsOnly, setFriendsOnly] = useState(true);
  const [historyBoardId, setHistoryBoardId] = useState<string | null>(null);
  const [settingsDraft, setSettingsDraft] = useState<SettingsDraft | null>(null);
  const [settingsStatus, setSettingsStatus] = useState<SettingsStatus>({ error: false, message: "" });

  return (
    <WorkspacePresentationContext value={{
      friendsQuery,
      setFriendsQuery,
      friendsOnly,
      setFriendsOnly,
      historyBoardId,
      setHistoryBoardId,
      settingsDraft,
      setSettingsDraft,
      settingsStatus,
      setSettingsStatus,
    }}>
      {children}
    </WorkspacePresentationContext>
  );
}

export function useWorkspacePresentation() {
  return useContext(WorkspacePresentationContext);
}
