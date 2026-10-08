export function useMaterialList(params: any) {
  return {
    list: [],
    error: null,
    total: 0,
    isLoading: false,
    pages: [],
    pagesCount: 0,
    currentPage: 1,
    setCurrentPage: () => {},
    pageSize: 10,
    setPageSize: () => {},
    setQueryParams: () => {},
  };
}
