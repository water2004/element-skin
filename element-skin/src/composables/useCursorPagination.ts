import { ref, computed } from 'vue'

export interface CursorPageResponse<T> {
	items: T[]
	has_next: boolean
	next_cursor: string | null
	page_size: number
	total?: number  // 兼容旧API
}

export function useCursorPagination<T>(defaultLimit = 20) {
	const items = ref<T[]>([])
	const hasNext = ref(false)
	const hasPrev = ref(false)
	const currentCursor = ref<string | null>(null)
	const nextCursor = ref<string | null>(null)
	const isLoading = ref(false)
	const limit = ref(defaultLimit)
	const cursorStack = ref<(string | null)[]>([])  // 栈式结构，用于向前翻

	const canGoNext = computed(() => hasNext.value && !isLoading.value)
	const canGoPrev = computed(() => hasPrev.value && !isLoading.value)

	const goToNextPage = async (fetchFunction: (cursor: string | null, limit: number) => Promise<CursorPageResponse<T>>) => {
		if (!hasNext.value || !nextCursor.value) return
		isLoading.value = true
		try {
			// 保存当前游标用于向后翻
			cursorStack.value.push(currentCursor.value)
			currentCursor.value = nextCursor.value
			const response = await fetchFunction(nextCursor.value, limit.value)
			setPageData(response)
		} finally {
			isLoading.value = false
		}
	}

	const goToPrevPage = async (fetchFunction: (cursor: string | null, limit: number) => Promise<CursorPageResponse<T>>) => {
		if (cursorStack.value.length === 0) return
		isLoading.value = true
		try {
			const prevCursorValue = cursorStack.value.pop() ?? null
			currentCursor.value = prevCursorValue
			const response = await fetchFunction(prevCursorValue, limit.value)
			setPageData(response)
		} finally {
			isLoading.value = false
		}
	}

	const setPageData = (response: CursorPageResponse<T>) => {
		items.value = response.items
		hasNext.value = response.has_next
		nextCursor.value = response.next_cursor
		hasPrev.value = cursorStack.value.length > 0
	}

	const reset = () => {
		items.value = []
		hasNext.value = false
		hasPrev.value = false
		currentCursor.value = null
		nextCursor.value = null
		cursorStack.value = []
	}

	return {
		items,
		hasNext,
		hasPrev,
		canGoNext,
		canGoPrev,
		currentCursor,
		nextCursor,
		isLoading,
		limit,
		goToNextPage,
		goToPrevPage,
		setPageData,
		reset,
	}
}
