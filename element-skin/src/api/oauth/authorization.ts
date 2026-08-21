import apiClient from '../client'
import type {
  DeviceAuthorizationDetails,
  OAuthAuthorizationApproval,
  OAuthAuthorizationDetails,
  OAuthAuthorizationRequest,
  OpenIDConfiguration,
} from './types'

export function getOpenIDConfiguration() {
  return apiClient.get<OpenIDConfiguration>('/.well-known/openid-configuration')
}

export function getOAuthAuthorizationDetails(params: OAuthAuthorizationRequest) {
  return apiClient.get<OAuthAuthorizationDetails>('/oauth/authorize', {
    params,
  })
}

export function approveOAuthAuthorization(payload: OAuthAuthorizationRequest) {
  return apiClient.post<OAuthAuthorizationApproval>('/oauth/authorize', payload)
}

export function getDeviceAuthorization(userCode: string) {
  return apiClient.get<DeviceAuthorizationDetails>('/oauth/device', {
    params: { user_code: userCode },
  })
}

export function decideDeviceAuthorization(userCode: string, approve: boolean) {
  return apiClient.post<void>('/oauth/device', {
    user_code: userCode,
    approve,
  })
}
