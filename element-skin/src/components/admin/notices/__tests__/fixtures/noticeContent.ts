export const noticePreviewContent = {
  short: '简短预览',
  long: Array.from({ length: 60 }, (_, index) => `预览第 ${index + 1} 段`).join('\n\n'),
  paragraphCount: 60,
  lastParagraph: '预览第 60 段',
}
