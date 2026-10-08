import React, { useState, useEffect } from "react";
import {
  Icon,
  Flex,
  Text,
  Box,
  Image,
  Input,
  IconButton,
} from "@chakra-ui/react";
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ArrowForwardIcon,
} from "@chakra-ui/icons";
import { format, endOfDay, startOfDay, getUnixTime } from "date-fns";
import { AiFillCloseCircle } from "react-icons/ai";
import DatePicker from "react-datepicker";
import "react-datepicker/dist/react-datepicker.css";
import { resolveResponsiveControlLayout } from "@/components/layout/responsive-layout.mjs";

export type DateRangePickerType = {
  name: string;
  label?: string;
  props?: any;
  // eslint-disable-next-line no-unused-vars
  onChange: (value: any) => void;
  isClear: boolean;
};

const CustomDatePickerInput = React.forwardRef<HTMLInputElement>(
  (props, ref) => {
    return (
      <Input
        ref={ref}
        size="sm"
        variant="flushed"
        textAlign="center"
        color="workbench.text"
        borderColor="transparent"
        focusBorderColor="primary.400"
        _placeholder={{ color: "workbench.muted" }}
        h="full"
        {...props}
      />
    );
  },
);
export function CustomDatePickerHeader(props) {
  const {
    date,
    decreaseYear,
    increaseYear,
    decreaseMonth,
    increaseMonth,
    nextYearButtonDisabled,
    prevYearButtonDisabled,
    prevMonthButtonDisabled,
    nextMonthButtonDisabled,
  } = props;
  const ICON_STYLE = {
    color: "neutral.900",
    size: "xs",
    variant: "unstyled",
    _hover: { color: "neutral.600" },
  };
  return (
    <Flex justify="space-between" align="center">
      <IconButton
        aria-label=""
        icon={<ArrowLeftIcon />}
        onClick={decreaseYear}
        disabled={prevYearButtonDisabled}
        {...ICON_STYLE}
      />
      <IconButton
        aria-label=""
        icon={<ChevronLeftIcon boxSize={4} />}
        onClick={decreaseMonth}
        disabled={prevMonthButtonDisabled}
        {...ICON_STYLE}
      />
      <Text fontWeight="medium" fontSize="md">
        {format(date, "yyyy年 MM月")}
      </Text>
      <IconButton
        aria-label=""
        icon={<ChevronRightIcon boxSize={4} />}
        onClick={increaseMonth}
        disabled={nextMonthButtonDisabled}
        {...ICON_STYLE}
      />
      <IconButton
        aria-label=""
        icon={<ArrowRightIcon />}
        onClick={increaseYear}
        disabled={nextYearButtonDisabled}
        {...ICON_STYLE}
      />
    </Flex>
  );
}
function DateRangePicker({
  name,
  label,
  props,
  onChange,
  isClear,
}: DateRangePickerType) {
  const responsiveLayout = resolveResponsiveControlLayout(props, "auto");
  const [startDate, setStartDate] = useState<Date | null>();
  const [endDate, setEndDate] = useState<Date | null>();
  useEffect(() => {
    if (isClear) {
      setStartDate(null);
      setEndDate(null);
    }
  }, [isClear]);
  const handleStartDate = (date) => {
    if (date && endDate) {
      onChange({
        [name]: [getUnixTime(date), getUnixTime(endDate)],
      });
    }
    setStartDate(date);
  };
  const handleEndDate = (date) => {
    if (date && startDate) {
      onChange({
        [name]: [getUnixTime(startDate), getUnixTime(date)],
      });
    }
    setEndDate(date);
  };
  const clear = () => {
    onChange({
      [name]: [],
    });
  };
  return (
    <Flex
      {...props}
      w={responsiveLayout.w}
      ml={responsiveLayout.ml}
      mr={responsiveLayout.mr}
      direction={{ base: "column", sm: "row" }}
      align={{ base: "stretch", sm: "center" }}
      gap={{ base: 1, sm: 0 }}
    >
      {label && (
        <Text
          pr={{ base: 0, sm: 2 }}
          fontSize="sm"
          fontWeight="600"
          color="workbench.muted"
          minW={{ base: 0, sm: "80px" }}
          textAlign={{ base: "left", sm: "right" }}
        >
          {label}:
        </Text>
      )}
      <Flex
        bg="workbench.paper"
        borderWidth="1px"
        borderRadius={props?.borderRadius || "10px"}
        h={props?.h || "11"}
        w={{ base: "full", md: "auto" }}
        borderColor="workbench.line"
        align="center"
        justify="space-between"
        pr={1}
        _hover={{ cursor: "pointer", borderColor: "neutral.300" }}
        sx={{
          ".react-datepicker__header": {
            bgColor: "white",
            borderColor: "neutral.100",
            p: 3,
          },
          ".react-datepicker-wrapper, .react-datepicker__input-container": {
            w: "full",
          },
          ".react-datepicker-popper": {
            zIndex: 1000,
          },
          ".react-datepicker": {
            border: "none",
            borderRadius: "2px",
            boxShadow: "0px 4px 12px 0px rgba(0,0,0,0.10)",
          },
          ".react-datepicker__day--today": {
            bgColor: "white",
          },
          ".react-datepicker__day": {
            fontSize: "13px",
            borderRadius: "2px",
            color: "rgba(0,0,0,0.65)",
            cursor: "pointer",
          },
          ".react-datepicker__day--disabled": {
            color: "neutral.300",
            cursor: "not-allowed",
            _hover: {
              bgColor: "unset",
              color: "neutral.300",
            },
          },
          ".react-datepicker__day-names": {
            mt: 1,
          },
          ".react-datepicker__week": {
            mb: 1,
          },
          ".react-datepicker__day--in-selecting-range": {
            bgColor: "#F4F6FF",
          },
          ".react-datepicker__day--in-range": {
            bgColor: "#F4F6FF",
          },
          ".react-datepicker__day--selecting-range-end, .react-datepicker__day--selected, .react-datepicker__day--selecting-range-start":
            {
              bgColor: "primary.600",
              color: "neutral.800",
              _hover: {
                bgColor: "primary.600",
                color: "neutral.800",
              },
            },
        }}
      >
        <Box h="full" flex={1} minW={0} display="flex" alignItems="center">
          <DatePicker
            startDate={startDate}
            endDate={endDate}
            selected={startDate}
            onChange={(date) => handleStartDate(startOfDay(date))}
            dateFormat="yyyy/MM/dd"
            placeholderText="开始日期"
            showPopperArrow={false}
            popperPlacement="bottom-start"
            selectsStart
            maxDate={endDate || new Date()}
            customInput={<CustomDatePickerInput />}
            renderCustomHeader={CustomDatePickerHeader}
          />
        </Box>
        {/* <Divider mx={2} w="10px" borderColor="neutral.300" /> */}
        <Icon as={ArrowForwardIcon} color="rgba(0,0,0,0.25)" />
        <Box h="full" flex={1} minW={0} display="flex" alignItems="center">
          <DatePicker
            selected={endDate}
            startDate={startDate}
            endDate={endDate}
            onChange={(date) => handleEndDate(endOfDay(date))}
            selectsEnd
            dateFormat="yyyy/MM/dd"
            placeholderText="结束日期"
            minDate={startDate}
            maxDate={new Date()}
            showPopperArrow={false}
            showFullMonthYearPicker
            popperPlacement="bottom-end"
            customInput={<CustomDatePickerInput />}
            renderCustomHeader={CustomDatePickerHeader}
          />
        </Box>
        {!startDate && !endDate ? (
          <Image boxSize={5} src="/images/calendar.png" alt="筛选日期" />
        ) : (
          <Icon
            color="workbench.muted"
            onClick={() => {
              setStartDate(null);
              setEndDate(null);
              clear();
            }}
            as={AiFillCloseCircle}
          />
        )}
      </Flex>
    </Flex>
  );
}

export default DateRangePicker;
