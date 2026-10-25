export const pageSizeCases = [
  { label: 'missing', stored: null, expected: 20 },
  { label: 'empty', stored: '', expected: 20 },
  { label: 'whitespace', stored: ' \t ', expected: 20 },
  { label: 'corrupt', stored: 'twenty', expected: 20 },
  { label: 'non-finite', stored: 'Infinity', expected: 20 },
  { label: 'minimum', stored: '1', expected: 1 },
  { label: 'maximum', stored: '100', expected: 100 },
  { label: 'below minimum', stored: '0', expected: 1 },
  { label: 'negative', stored: '-4', expected: 1 },
  { label: 'above maximum', stored: '500', expected: 100 },
  { label: 'fractional', stored: '10.6', expected: 11 },
] as const
