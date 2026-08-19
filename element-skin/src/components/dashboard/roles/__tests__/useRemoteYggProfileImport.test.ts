import { describe, expect, it, vi } from 'vitest'
import type { YggdrasilImportResult } from '@/api/types'
import {
  remoteYggImportPayload,
  remoteYggImportResultMessage,
  selectedRemoteYggProfiles,
  useRemoteYggProfileImport,
  type RemoteYggProfile,
} from '../useRemoteYggProfileImport'

function importResult(overrides: Partial<YggdrasilImportResult> = {}): YggdrasilImportResult {
  return {
    items: [],
    success_count: 0,
    failure_count: 0,
    failed: [],
    ...overrides,
  }
}

describe('useRemoteYggProfileImport helpers', () => {
  it('selects remote profiles in source order and ignores missing ids', () => {
    const profiles: RemoteYggProfile[] = [
      { id: 'p1', name: 'Steve' },
      { id: 'p2', name: 'Alex' },
      { id: 'p3', name: 'Herobrine' },
    ]

    expect(selectedRemoteYggProfiles(profiles, ['missing', 'p3', 'p1'])).toEqual([
      { id: 'p1', name: 'Steve' },
      { id: 'p3', name: 'Herobrine' },
    ])
  })

  it('builds exact remote ygg import payload', () => {
    expect(
      remoteYggImportPayload([
        { id: 'p1', name: 'Steve' },
        { id: 'p2', name: 'Alex' },
      ]),
    ).toEqual([
      { profile_id: 'p1', profile_name: 'Steve' },
      { profile_id: 'p2', profile_name: 'Alex' },
    ])
  })

  it('formats exact success and partial failure import messages', () => {
    expect(remoteYggImportResultMessage({ success_count: 2, failure_count: 0 })).toEqual({
      type: 'success',
      message: '成功导入 2 个角色',
    })
    expect(remoteYggImportResultMessage({ success_count: 1, failure_count: 3 })).toEqual({
      type: 'warning',
      message: '已导入 1 个角色，3 个失败',
    })
  })
})

describe('useRemoteYggProfileImport', () => {
  it('rejects incomplete preview input without calling api', async () => {
    const getProfiles = vi.fn()
    const warning = vi.fn()
    const state = useRemoteYggProfileImport({ getProfiles, warning })

    await state.getYggProfiles()

    expect(warning).toHaveBeenCalledTimes(1)
    expect(warning).toHaveBeenCalledWith('请填写完整信息')
    expect(getProfiles).not.toHaveBeenCalled()
    expect(state.yggStep.value).toBe('input')
    expect(state.yggLoading.value).toBe(false)
  })

  it('loads profiles and selects every returned profile exactly', async () => {
    const getProfiles = vi.fn().mockResolvedValue({
      data: {
        items: [
          { id: 'p1', name: 'Steve' },
          { id: 'p2', name: 'Alex' },
        ],
      },
    })
    const state = useRemoteYggProfileImport({ getProfiles })
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggUsername.value = 'steve@example.com'
    state.yggPassword.value = 'password'

    await state.getYggProfiles()

    expect(getProfiles).toHaveBeenCalledTimes(1)
    expect(getProfiles).toHaveBeenCalledWith({
      api_url: 'https://remote.example/api',
      username: 'steve@example.com',
      password: 'password',
    })
    expect(state.yggProfiles.value).toEqual([
      { id: 'p1', name: 'Steve' },
      { id: 'p2', name: 'Alex' },
    ])
    expect(state.selectedYggProfiles.value).toEqual(['p1', 'p2'])
    expect(state.yggStep.value).toBe('select')
    expect(state.yggLoading.value).toBe(false)
  })

  it('keeps input step and warns when preview returns no profiles', async () => {
    const warning = vi.fn()
    const state = useRemoteYggProfileImport({
      warning,
      getProfiles: vi.fn().mockResolvedValue({ data: { items: [] } }),
    })
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggUsername.value = 'empty@example.com'
    state.yggPassword.value = 'password'

    await state.getYggProfiles()

    expect(warning).toHaveBeenCalledWith('该账户下没有角色')
    expect(state.yggProfiles.value).toEqual([])
    expect(state.selectedYggProfiles.value).toEqual([])
    expect(state.yggStep.value).toBe('input')
  })

  it('imports selected profiles, refreshes owner state, hides dialog and resets form exactly', async () => {
    const importProfiles = vi.fn().mockResolvedValue({
      data: importResult({ success_count: 2, failure_count: 0 }),
    })
    const onImported = vi.fn()
    const success = vi.fn()
    const state = useRemoteYggProfileImport({ importProfiles, onImported, success })
    state.showYggImportDialog.value = true
    state.yggStep.value = 'select'
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggUsername.value = 'steve@example.com'
    state.yggPassword.value = 'password'
    state.yggProfiles.value = [
      { id: 'p1', name: 'Steve' },
      { id: 'p2', name: 'Alex' },
      { id: 'p3', name: 'Unused' },
    ]
    state.selectedYggProfiles.value = ['p2', 'p1']

    await state.importYggProfile()

    expect(importProfiles).toHaveBeenCalledTimes(1)
    expect(importProfiles).toHaveBeenCalledWith({
      api_url: 'https://remote.example/api',
      profiles: [
        { profile_id: 'p1', profile_name: 'Steve' },
        { profile_id: 'p2', profile_name: 'Alex' },
      ],
    })
    expect(success).toHaveBeenCalledWith('成功导入 2 个角色')
    expect(onImported).toHaveBeenCalledTimes(1)
    expect(state.showYggImportDialog.value).toBe(false)
    expect(state.yggStep.value).toBe('input')
    expect(state.yggApiUrl.value).toBe('')
    expect(state.yggUsername.value).toBe('')
    expect(state.yggPassword.value).toBe('')
    expect(state.yggProfiles.value).toEqual([])
    expect(state.selectedYggProfiles.value).toEqual([])
    expect(state.yggLoading.value).toBe(false)
  })

  it('reports partial import failures with warning text', async () => {
    const warning = vi.fn()
    const state = useRemoteYggProfileImport({
      warning,
      importProfiles: vi.fn().mockResolvedValue({
        data: importResult({ success_count: 1, failure_count: 1 }),
      }),
    })
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggProfiles.value = [{ id: 'p1', name: 'Steve' }]
    state.selectedYggProfiles.value = ['p1']

    await state.importYggProfile()

    expect(warning).toHaveBeenCalledWith('已导入 1 个角色，1 个失败')
  })

  it('does not call import api when no profile is selected', async () => {
    const importProfiles = vi.fn()
    const state = useRemoteYggProfileImport({ importProfiles })
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggProfiles.value = [{ id: 'p1', name: 'Steve' }]
    state.selectedYggProfiles.value = []

    await state.importYggProfile()

    expect(importProfiles).not.toHaveBeenCalled()
    expect(state.yggLoading.value).toBe(false)
  })

  it('reports preview and import errors exactly', async () => {
    const error = vi.fn()
    const state = useRemoteYggProfileImport({
      error,
      getProfiles: vi.fn().mockRejectedValue({
        response: {
          data: {
            error: { object: 'remote_profile', operation: 'fetch', reason: 'unavailable' },
          },
        },
      }),
      importProfiles: vi.fn().mockRejectedValue({ message: 'import failed' }),
    })
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggUsername.value = 'steve@example.com'
    state.yggPassword.value = 'password'

    await state.getYggProfiles()
    state.yggProfiles.value = [{ id: 'p1', name: 'Steve' }]
    state.selectedYggProfiles.value = ['p1']
    await state.importYggProfile()

    expect(error).toHaveBeenNthCalledWith(1, '获取失败: 相关服务暂时不可用')
    expect(error).toHaveBeenNthCalledWith(2, '导入失败: import failed')
    expect(state.yggLoading.value).toBe(false)
  })

  it('closes and resets dialog unless a request is running', () => {
    const done = vi.fn()
    const state = useRemoteYggProfileImport()
    state.showYggImportDialog.value = true
    state.yggStep.value = 'select'
    state.yggApiUrl.value = 'https://remote.example/api'
    state.yggProfiles.value = [{ id: 'p1', name: 'Steve' }]
    state.selectedYggProfiles.value = ['p1']

    state.yggLoading.value = true
    state.handleYggDialogClose(done)
    expect(state.showYggImportDialog.value).toBe(true)
    expect(done).not.toHaveBeenCalled()

    state.yggLoading.value = false
    state.handleYggDialogClose(done)
    expect(state.showYggImportDialog.value).toBe(false)
    expect(state.yggStep.value).toBe('input')
    expect(state.yggApiUrl.value).toBe('')
    expect(state.yggProfiles.value).toEqual([])
    expect(state.selectedYggProfiles.value).toEqual([])
    expect(done).toHaveBeenCalledTimes(1)
  })
})
