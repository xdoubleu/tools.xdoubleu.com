import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_connect'
import type {
  GetSettingsResponse,
  AddLabelPresetRequest,
  RemoveLabelPresetRequest,
  UpdateLabelColorRequest,
  AddURLPatternRequest,
  RemoveURLPatternRequest,
  AddSectionRequest,
  RemoveSectionRequest,
  AddPolicyRequest,
  UpdatePolicyRequest,
  RemovePolicyRequest,
  AddWorkspaceRequest,
  DeleteWorkspaceRequest,
  SetActiveWorkspaceRequest,
  UpdateArchiveSettingsRequest
} from '@/lib/gen/todos/v1/settings_pb'

export function useTodoSettings() {
  const client = createServiceClient(SettingsService)
  return useSWR<GetSettingsResponse>('/todos/settings', () => client.getSettings({}))
}

export function useAddLabelPreset() {
  const client = createServiceClient(SettingsService)
  return (req: AddLabelPresetRequest) => client.addLabelPreset(req)
}

export function useRemoveLabelPreset() {
  const client = createServiceClient(SettingsService)
  return (req: RemoveLabelPresetRequest) => client.removeLabelPreset(req)
}

export function useUpdateLabelColor() {
  const client = createServiceClient(SettingsService)
  return (req: UpdateLabelColorRequest) => client.updateLabelColor(req)
}

export function useAddURLPattern() {
  const client = createServiceClient(SettingsService)
  return (req: AddURLPatternRequest) => client.addURLPattern(req)
}

export function useRemoveURLPattern() {
  const client = createServiceClient(SettingsService)
  return (req: RemoveURLPatternRequest) => client.removeURLPattern(req)
}

export function useAddSection() {
  const client = createServiceClient(SettingsService)
  return (req: AddSectionRequest) => client.addSection(req)
}

export function useRemoveSection() {
  const client = createServiceClient(SettingsService)
  return (req: RemoveSectionRequest) => client.removeSection(req)
}

export function useAddPolicy() {
  const client = createServiceClient(SettingsService)
  return (req: AddPolicyRequest) => client.addPolicy(req)
}

export function useUpdatePolicy() {
  const client = createServiceClient(SettingsService)
  return (req: UpdatePolicyRequest) => client.updatePolicy(req)
}

export function useRemovePolicy() {
  const client = createServiceClient(SettingsService)
  return (req: RemovePolicyRequest) => client.removePolicy(req)
}

export function useAddWorkspace() {
  const client = createServiceClient(SettingsService)
  return (req: AddWorkspaceRequest) => client.addWorkspace(req)
}

export function useDeleteWorkspace() {
  const client = createServiceClient(SettingsService)
  return (req: DeleteWorkspaceRequest) => client.deleteWorkspace(req)
}

export function useSetActiveWorkspace() {
  const client = createServiceClient(SettingsService)
  return (req: SetActiveWorkspaceRequest) => client.setActiveWorkspace(req)
}

export function useUpdateArchiveSettings() {
  const client = createServiceClient(SettingsService)
  return (req: UpdateArchiveSettingsRequest) => client.updateArchiveSettings(req)
}
