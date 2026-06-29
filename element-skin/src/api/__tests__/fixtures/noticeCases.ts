import { dismissNotice, getNotice, getNotices, markNoticeRead } from '../../notices'
import type { ApiCase } from './types'

export function noticeApiCases(): ApiCase[] {
  return [
    {
      name: 'getNotices gets notice params',
      method: 'get',
      call: () =>
        getNotices({
          cursor: 'notice-cursor',
          limit: 10,
          type: 'announcement',
          include_read: true,
        }),
      args: [
        '/v1/notifications',
        {
          params: {
            cursor: 'notice-cursor',
            limit: 10,
            type: 'announcement',
            include_read: true,
          },
        },
      ],
    },
    {
      name: 'getNotice gets notice detail',
      method: 'get',
      call: () => getNotice('notice-1'),
      args: ['/v1/notifications/notice-1'],
    },
    {
      name: 'markNoticeRead posts read endpoint',
      method: 'post',
      call: () => markNoticeRead('notice-1'),
      args: ['/v1/notifications/notice-1/read'],
    },
    {
      name: 'dismissNotice posts dismiss endpoint',
      method: 'post',
      call: () => dismissNotice('notice-1'),
      args: ['/v1/notifications/notice-1/dismiss'],
    },
  ]
}
