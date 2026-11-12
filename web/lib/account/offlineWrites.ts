import { isMessage } from '@bufbuild/protobuf'
import { AuthService, GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'

export const setTabPinsWrite = defineOfflineWrite({
  method: AuthService.method.setTabPins,
  apply: (key, data, req) =>
    key === swrKeys.currentUser && isMessage(data, GetCurrentUserResponseSchema)
      ? { ...data, tabPins: req.tabPins, tabPinsCustomised: true }
      : data,
  describe: () => 'Change the tab bar apps',
  revalidate: swrKeys.currentUser,
  // The cache already shows the saved pins.
  revalidateOnSuccess: false,
  coalesce: () => 'tab-pins'
})

export const accountWrites = [setTabPinsWrite]
