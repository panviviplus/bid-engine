/* eslint-disable react-hooks/exhaustive-deps */
import { useEffect, useState, useMemo } from "react";
import { useRouter, usePathname } from "next/navigation";
import { createContext, useContext } from "use-context-selector";
import { USER_ROLE } from "@/types/user";
import useSWR from "swr";

const AppContext = createContext({
  userProfile: {
    id: "",
    name: "",
    menu: [],
    nickname: "",
    email: "",
    avatar: "",
    avatarUrl: "",
    avatarFile: "",
    company_name: "",
    companyName: "",
    companyId: 0,
    is_password_set: false,
  },
});

export function AppContextProvider({ children }) {
  const router = useRouter();
  const pathname = usePathname();
  const shouldFetchUserInfo =
    pathname &&
    pathname !== "/signin" &&
    pathname !== "/signup" &&
    !pathname.startsWith("/pdf-preview");

  const { data, mutate } = useSWR(shouldFetchUserInfo ? "/api/info" : null, {
    refreshInterval: 10000,
  });

  useEffect(() => {
    if (!shouldFetchUserInfo) return;
    mutate();
  }, [pathname, mutate, shouldFetchUserInfo]);

  const [userProfile, setUserProfile] = useState({});

  useEffect(() => {
    if (!shouldFetchUserInfo) return;
    if (data) {
      if (data?.code === 0 && data?.data) {
        setUserProfile({
          ...data?.data,
          id: data?.data?.user_id,
          name: data?.data?.name,
          nickname: data?.data?.name,
          companyName: data?.data?.company,
          companyId: data?.data?.companyId,
          avatarFile: data?.data?.avatar_file,
          avatarUrl: data?.data?.avatar_url?.replace(
            "host.docker.internal",
            "localhost",
          ),
          isAdmin: data?.data?.userRole === USER_ROLE.ADMIN,
          isCompanyOwner: data?.data?.userRole === USER_ROLE.OWNER.COMPANY,
          isMember: data?.data?.userRole === USER_ROLE.MEMBER,
        });
        if (data?.data?.userRole === "") {
          return router.replace("/403");
        }
      } else {
        router.replace("/signin");
      }
    }
  }, [router, data, shouldFetchUserInfo]);

  const values = useMemo(() => {
    return {
      userProfile: userProfile || {},
      mutateUserProfile: mutate,
    };
  }, [userProfile, mutate]);

  return <AppContext.Provider value={values}>{children}</AppContext.Provider>;
}

export const useAppContext = () => useContext(AppContext);

export default AppContext;
