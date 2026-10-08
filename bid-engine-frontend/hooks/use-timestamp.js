"use client";

import { useCallback } from "react";
import dayjs from "dayjs";

const useTimestamp = () => {
  const formatTime = useCallback((value, format) => {
    return dayjs.unix(value).format(format);
  }, []);

  return { formatTime };
};

export default useTimestamp;
