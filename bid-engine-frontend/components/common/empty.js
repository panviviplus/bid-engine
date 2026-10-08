import { Flex, Text, Image } from "@chakra-ui/react";
import EmptySVG from "../svg/error/empty";
import Error500SVG from "../svg/error/error-500";
import Error404SVG from "../svg/error/error-404";
import ErrorSVG from "../svg/error/error-general";

function Empty({ message, type, children, ...props }) {
  return (
    <Flex
      direction="column"
      flex={1}
      align="center"
      justify="center"
      px={4}
      {...props}
    >
      {type === "empty" && <EmptySVG w={180} h={180} />}
      {type === "check" && (
        <Image w={180} h={180} alt="empty" src="/images/check.svg" />
      )}
      {type === "error-500" && <Error500SVG w={210} h={210} />}
      {type === "error" && <ErrorSVG w={210} h={210} />}
      {type === "error-404" && <Error404SVG w={210} h={200} />}
      {type === "bid-applying" && (
        <Image
          w={{ base: "80px", md: "160px" }}
          h={{ base: "80px", md: "160px" }}
          src="/images/empty/bid-apply.svg"
          alt="bid-apply"
        />
      )}
      {type === "bid-forbidden" && (
        <Image
          w={160}
          h={160}
          src="/images/empty/bid-forbidden.svg"
          alt="bid-forbidden"
        />
      )}
      {type === "bid-file-empty" && (
        <Image
          w={160}
          h={160}
          src="/images/empty/bid-file-empty.svg"
          alt="bid-file-empty"
        />
      )}
      {type === "bid-chat-empty" && (
        <Image
          w={160}
          h={160}
          src="/images/empty/bid-chat-empty.svg"
          alt="bid-chat-empty"
        />
      )}
      {type === "bid-error" && (
        <Image
          w={{ base: "80px", md: "160px" }}
          h={{ base: "80px", md: "160px" }}
          src="/images/empty/bid-error.svg"
          alt="bid-error"
        />
      )}
      {type === "not-login" && (
        <Image w={160} h={160} src="/images/not-login.svg" alt="not-login" />
      )}
      <Text
        color="neutral.500"
        fontSize={{ base: "xs", md: "sm" }}
        textAlign="center"
      >
        {message}
      </Text>
      {children}
    </Flex>
  );
}

export default Empty;
