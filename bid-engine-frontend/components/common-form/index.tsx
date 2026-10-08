import { useState } from "react";
import { Button, Icon } from "@chakra-ui/react";
import { Field, Form, Formik } from "formik";
import Input, { InputType } from "./components/Input";
import DateRangePicker, {
  DateRangePickerType,
} from "./components/data-range-picker";
import Select, { SelectType } from "./components/Select";
import { DeleteIcon } from "@chakra-ui/icons";

export type FormItemType = {
  name: string;
  label?: string;
  icon?: boolean;
  placeholder?: string;
  type: "input" | "select" | "date_range" | "select_multi" | "select_paging";
  props?: any;
  defaultValue?: any;
};
type FormOptionsType = {
  [key: string]: SelectType["options"];
};
type ItemProps = {
  input: InputType;
  select: SelectType;
  date_range: any;
  select_multi: any;
  select_paging: any;
};
type CommonFormType = {
  isLoading?: boolean;
  isClearBtn?: boolean;
  formItems: FormItemType[];
  formOptions?: FormOptionsType;
  defaultFormValues?: {
    [key: string]: any;
  };
  clearBtnStyle?: any;
  // eslint-disable-next-line no-unused-vars
  onChange?: (formValues: any) => void;
  // eslint-disable-next-line no-unused-vars
  onFormConfirm?: (formValues: any) => void;
  props?: any;
};

function CommonForm({
  isLoading,
  isClearBtn,
  formItems,
  formOptions,
  defaultFormValues,
  clearBtnStyle,
  onChange,
  onFormConfirm,
  props,
}: CommonFormType) {
  const [formValues, setFormValues] = useState({ ...defaultFormValues });
  const [isClear, setIsClear] = useState(false);
  const formChange = (value) => {
    setFormValues({ ...formValues, ...value });
    onChange?.({ ...formValues, ...value });
    setIsClear(false);
  };
  const formClear = () => {
    setIsClear(true);
    setFormValues({ ...defaultFormValues });
    onChange?.({});
  };
  const formConfirm = () => {
    onFormConfirm?.(formValues);
  };
  function renderInput({ name, label, props, icon, placeholder }: InputType) {
    return (
      <Input
        name={name}
        label={label}
        icon={icon}
        placeholder={placeholder}
        props={props}
        onChange={formChange}
        isClear={isClear}
      />
    );
  }
  function renderDateRangePicker({ name, label, props }: DateRangePickerType) {
    return (
      <DateRangePicker
        name={name}
        label={label}
        props={props}
        onChange={formChange}
        isClear={isClear}
      />
    );
  }
  function renderSelect({
    options,
    label,
    placeholder,
    type,
    props,
    name,
    paginate,
  }: SelectType) {
    return (
      <Select
        name={name}
        options={options}
        label={label}
        placeholder={placeholder}
        defaultValue={defaultFormValues?.[name]}
        type={type}
        props={props}
        onChange={(value) => formChange(value)}
        isClear={isClear}
        paginate={paginate}
      />
    );
  }
  function rederFormItems<T extends keyof ItemProps>(
    type: T,
    props: ItemProps[T],
  ) {
    let item: any = null;
    if (type === "input") {
      item = renderInput(props);
    } else if (type === "select") {
      item = renderSelect(props);
    } else if (type === "date_range") {
      item = renderDateRangePicker(props);
    } else if (type === "select_multi") {
      item = renderSelect(props);
    } else if (type === "select_paging") {
      item = renderSelect(props);
    }
    return item;
  }

  const formatOptions = (options: any[]) => {
    if (options.length) {
      return options;
    }
    return [];
  };

  return (
    <Formik
      initialValues={{ ...defaultFormValues, ...formValues }}
      onSubmit={(values, actions) => {
        console.log(6666, values, actions);
      }}
    >
      {() => (
        <Form
          style={{
            display: "flex",
            flexWrap: "wrap",
            width: "100%",
            gap: "var(--chakra-space-3)",
            ...props,
          }}
        >
          {formItems?.map((item) => {
            return (
              <Field key={item.name} name={item.name}>
                {() => {
                  return rederFormItems(item.type, {
                    options: formOptions?.[item.name]
                      ? formatOptions(formOptions[item.name])
                      : [],
                    ...item,
                  });
                }}
              </Field>
            );
          })}
          {!!onFormConfirm && (
            <Button
              ml="auto"
              isLoading={isLoading}
              bg="primary.600"
              minW="4.625rem"
              h={11}
              color="#ffffff"
              borderRadius="2px"
              fontSize="md"
              sx={clearBtnStyle}
              onClick={formConfirm}
            >
              查询
            </Button>
          )}
          {isClearBtn && (
            <Button
              ml="2"
              h={11}
              borderRadius="2px"
              fontSize="md"
              variant="outline"
              colorScheme="gray"
              borderColor="gray.200"
              leftIcon={<Icon as={DeleteIcon} />}
              sx={clearBtnStyle}
              onClick={formClear}
              _hover={{ borderColor: "primary.600", color: "primary.600", bg: "white" }}
            >
              重置
            </Button>
          )}
        </Form>
      )}
    </Formik>
  );
}

export default CommonForm;
