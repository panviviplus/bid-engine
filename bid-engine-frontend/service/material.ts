import useAxios from "axios-hooks";

const useMaterialTypes = () => {
  const [{ data, loading, error }, fetchTypes] = useAxios(
    { url: "/material/types", method: "GET" },
    { manual: true, useCache: false },
  );
  return {
    typeOptions: data?.data || [],
    typeLoading: loading,
    typeError: error,
    fetchTypes,
  };
};

const useMaterialCompanies = () => {
  const [{ data, loading, error }, fetchCompanies] = useAxios(
    { url: "/material/companies", method: "GET" },
    { manual: true, useCache: false },
  );
  return {
    companyOptions: data?.data || [],
    companyLoading: loading,
    companyError: error,
    fetchCompanies,
  };
};

const useMaterialUsers = () => {
  const [{ data, loading, error }, fetchUsers] = useAxios(
    { url: "/material/users", method: "GET" },
    { manual: true, useCache: false },
  );
  return {
    userOptions: data?.data || [],
    userLoading: loading,
    userError: error,
    fetchUsers,
  };
};

const useMaterialList = (params?: any) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/material/list", method: "GET", params },
    { manual: true, useCache: false },
  );
  return {
    listData: data?.data?.list || [],
    listTotal: data?.data?.total || 0,
    listLoading: loading,
    listError: error,
    fetchList,
  };
};

// 类型专属列表 hooks
const useQualificationList = (params?: any) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/material/qualification/list", method: "GET", params },
    { manual: true, useCache: false },
  );
  return { listData: data?.data?.list || [], listTotal: data?.data?.total || 0, listLoading: loading, listError: error, fetchList };
};

const usePerformanceList = (params?: any) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/material/performance/list", method: "GET", params },
    { manual: true, useCache: false },
  );
  return { listData: data?.data?.list || [], listTotal: data?.data?.total || 0, listLoading: loading, listError: error, fetchList };
};

const useTemplateList = (params?: any) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/material/template/list", method: "GET", params },
    { manual: true, useCache: false },
  );
  return { listData: data?.data?.list || [], listTotal: data?.data?.total || 0, listLoading: loading, listError: error, fetchList };
};

// 类型专属 CRUD hooks
const useQualificationAdd = () => {
  const [{ loading }, fetchAdd] = useAxios(
    { url: "/material/qualification/add", method: "POST" },
    { manual: true },
  );
  return { addLoading: loading, fetchAdd };
};
const useQualificationDetail = () => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    { method: "GET" },
    { manual: true, useCache: false },
  );
  return { detailData: data?.data || null, detailLoading: loading, detailError: error, fetchDetail };
};
const useQualificationUpdate = () => {
  const [{ loading }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { updateLoading: loading, fetchUpdate };
};
const useQualificationDelete = () => {
  const [{ loading }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, fetchDelete };
};

const usePerformanceAdd = () => {
  const [{ loading }, fetchAdd] = useAxios(
    { url: "/material/performance/add", method: "POST" },
    { manual: true },
  );
  return { addLoading: loading, fetchAdd };
};
const usePerformanceDetail = () => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    { method: "GET" },
    { manual: true, useCache: false },
  );
  return { detailData: data?.data || null, detailLoading: loading, detailError: error, fetchDetail };
};
const usePerformanceUpdate = () => {
  const [{ loading }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { updateLoading: loading, fetchUpdate };
};
const usePerformanceDelete = () => {
  const [{ loading }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, fetchDelete };
};

const useTemplateAdd = () => {
  const [{ loading }, fetchAdd] = useAxios(
    { url: "/material/template/add", method: "POST" },
    { manual: true },
  );
  return { addLoading: loading, fetchAdd };
};
const useTemplateDetail = () => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    { method: "GET" },
    { manual: true, useCache: false },
  );
  return { detailData: data?.data || null, detailLoading: loading, detailError: error, fetchDetail };
};
const useTemplateUpdate = () => {
  const [{ loading }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { updateLoading: loading, fetchUpdate };
};
const useTemplateDelete = () => {
  const [{ loading }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, fetchDelete };
};

const useMaterialAdd = () => {
  const [{ loading }, fetchAdd] = useAxios(
    { url: "/material/add", method: "POST" },
    { manual: true },
  );
  return { addLoading: loading, fetchAdd };
};

const useMaterialUpdate = () => {
  const [{ loading }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { updateLoading: loading, fetchUpdate };
};

const useMaterialDelete = () => {
  const [{ loading }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, fetchDelete };
};

const useMaterialDetail = () => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    { method: "GET" },
    { manual: true, useCache: false },
  );
  return {
    detailData: data?.data || null,
    detailLoading: loading,
    detailError: error,
    fetchDetail,
  };
};

const useMaterialFileAdd = () => {
  const [{ loading }, fetchFileAdd] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { fileAddLoading: loading, fetchFileAdd };
};

const useMaterialFileDelete = () => {
  const [{ loading }, fetchFileDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { fileDeleteLoading: loading, fetchFileDelete };
};

const useMaterialImageDelete = () => {
  const [{ loading }, fetchImageDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { imageDeleteLoading: loading, fetchImageDelete };
};

const useMaterialImageEdit = () => {
  const [{ loading }, fetchImageEdit] = useAxios(
    { url: "/material/image/edit", method: "PUT" },
    { manual: true },
  );
  return { imageEditLoading: loading, fetchImageEdit };
};

const useMaterialImageSortOrder = () => {
  const [{ loading }, fetchImageSortOrder] = useAxios(
    { url: "/material/image/sort-order", method: "PUT" },
    { manual: true },
  );
  return { imageSortLoading: loading, fetchImageSortOrder };
};

const useMaterialFileSortOrder = () => {
  const [{ loading }, fetchFileSortOrder] = useAxios(
    { url: "/material/file/sort-order", method: "PUT" },
    { manual: true },
  );
  return { fileSortLoading: loading, fetchFileSortOrder };
};

const useMaterialGallery = () => {
  const [{ data, loading, error }, fetchGallery] = useAxios(
    { url: "/material/gallery", method: "GET" },
    { manual: true, useCache: false },
  );
  return {
    galleryGroups: data?.data?.groups || [],
    galleryLoading: loading,
    galleryError: error,
    fetchGallery,
  };
};

const useMaterialGalleryUpload = () => {
  const [{ loading }, fetchGalleryUpload] = useAxios(
    { url: "/material/gallery/upload", method: "POST" },
    { manual: true },
  );
  return { galleryUploadLoading: loading, fetchGalleryUpload };
};

const useMaterialGalleryDelete = () => {
  const [{ loading }, fetchGalleryDelete] = useAxios(
    { url: "/material/gallery/images", method: "DELETE" },
    { manual: true },
  );
  return { galleryDeleteLoading: loading, fetchGalleryDelete };
};

const useMaterialGalleryMove = () => {
  const [{ loading }, fetchGalleryMove] = useAxios(
    { url: "/material/gallery/move", method: "PUT" },
    { manual: true },
  );
  return { galleryMoveLoading: loading, fetchGalleryMove };
};

const useMaterialOcrResults = (materialId?: number) => {
  const [{ data, loading, error }, fetchResults] = useAxios(
    {
      url: "/material/ocr-results",
      method: "GET",
      params: materialId ? { material_id: materialId } : undefined,
    },
    { manual: true, useCache: false },
  );
  return {
    ocrResults: data?.data || [],
    ocrLoading: loading,
    ocrError: error,
    fetchOcrResults: fetchResults,
  };
};

const useMaterialOcrRetry = () => {
  const [{ loading }, fetchRetry] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { ocrRetryLoading: loading, fetchOcrRetry: fetchRetry };
};

export {
  useMaterialTypes,
  useMaterialCompanies,
  useMaterialUsers,
  useMaterialList,
  useMaterialAdd,
  useMaterialUpdate,
  useMaterialDelete,
  useMaterialDetail,
  useMaterialFileAdd,
  useMaterialFileDelete,
  useMaterialImageDelete,
  useMaterialImageEdit,
  useMaterialImageSortOrder,
  useMaterialFileSortOrder,
  useMaterialGallery,
  useMaterialGalleryUpload,
  useMaterialGalleryDelete,
  useMaterialGalleryMove,
  useMaterialOcrResults,
  useMaterialOcrRetry,
  useQualificationList,
  usePerformanceList,
  useTemplateList,
  useQualificationAdd,
  useQualificationDetail,
  useQualificationUpdate,
  useQualificationDelete,
  usePerformanceAdd,
  usePerformanceDetail,
  usePerformanceUpdate,
  usePerformanceDelete,
  useTemplateAdd,
  useTemplateDetail,
  useTemplateUpdate,
  useTemplateDelete,
};
