export const passwordErrorFixtures = [
  { rules: ['required'], message: '请输入密码' },
  { rules: ['min_length'], message: '密码至少需要8个字符' },
  { rules: ['max_length'], message: '密码的UTF-8编码不能超过72字节' },
  {
    rules: ['composition'],
    message: '密码需包含小写字母、大写字母、数字、标点或符号中的至少两类',
  },
  {
    rules: ['min_length', 'composition'],
    message: '密码至少需要8个字符；密码需包含小写字母、大写字母、数字、标点或符号中的至少两类',
  },
]

export const malformedPasswordErrorFixtures = [
  { params: undefined, message: '密码不符合安全要求' },
  { params: { rules: 'min_length' }, message: '密码不符合安全要求' },
  { params: { rules: ['unknown', 'constructor', 123] }, message: '密码不符合安全要求' },
  { params: { rules: ['unknown', 'min_length', null] }, message: '密码至少需要8个字符' },
]
