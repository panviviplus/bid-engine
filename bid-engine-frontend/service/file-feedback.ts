import useAxios from "axios-hooks";

const useFeedbackOptions = () => {
  const [{ data, loading, error }, fetchFeedbackOptions] = useAxios(
    {
      url: "/feedback/options",
      method: "GET",
    },
    { manual: true, useCache: false },
  );

  return {
    optionsData: data?.data || {},
    optionsLoading: loading,
    optionsError: error,
    fetchFeedbackOptions,
  };
};

const useFeedbackRecords = (params) => {
  const [{ data, loading, error }, fetchFeedbackRecords] = useAxios(
    {
      url: "/feedback/records",
      method: "GET",
      params,
    },
    { manual: true, useCache: false },
  );

  return {
    recordsData: data?.data || { total: 0, items: [] },
    recordsLoading: loading,
    recordsError: error,
    fetchFeedbackRecords,
  };
};

const useAddFeedbackRecord = () => {
  const [{ loading, error }, fetchAddFeedbackRecord] = useAxios(
    {
      url: "/feedback/record",
      method: "POST",
    },
    { manual: true },
  );

  return {
    addRecordLoading: loading,
    addRecordError: error,
    fetchAddFeedbackRecord,
  };
};

const useGetFeedbackRecord = () => {
  const [{ data, loading, error }, fetchGetFeedbackRecord] = useAxios(
    {
      method: "GET",
    },
    { manual: true, useCache: false },
  );

  return {
    recordDetail: data?.data || {},
    recordDetailLoading: loading,
    recordDetailError: error,
    fetchGetFeedbackRecord,
  };
};

const useDeleteFeedbackRecord = () => {
  const [{ loading, error }, fetchDeleteFeedbackRecord] = useAxios(
    {
      method: "DELETE",
    },
    { manual: true },
  );

  return {
    deleteRecordLoading: loading,
    deleteRecordError: error,
    fetchDeleteFeedbackRecord,
  };
};

const useUpdateFeedbackStatus = () => {
  const [{ loading, error }, fetchUpdateFeedbackStatus] = useAxios(
    {
      method: "PUT",
    },
    { manual: true },
  );

  return {
    updateStatusLoading: loading,
    updateStatusError: error,
    fetchUpdateFeedbackStatus,
  };
};

export {
  useFeedbackOptions,
  useFeedbackRecords,
  useAddFeedbackRecord,
  useGetFeedbackRecord,
  useDeleteFeedbackRecord,
  useUpdateFeedbackStatus,
};
