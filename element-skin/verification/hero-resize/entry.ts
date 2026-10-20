import { appStorage } from '../../src/storage'

const theme = new URLSearchParams(location.search).get('theme')
if (theme === 'light' || theme === 'dark') appStorage.theme.set(theme)
appStorage.easterEgg.setDisabled(true)

// Mount the real application after selecting the manual verification scenario.
void import('../../src/main')
