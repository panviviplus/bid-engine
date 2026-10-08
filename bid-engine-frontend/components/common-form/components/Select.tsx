import SelectBase from "./SelectBase";
import SelectPaging, { PaginateConfig } from "./SelectPaging";

type OptionItem = { label: string; value: number | string };
type OptionsType = OptionItem[];
export type SelectType = {
  options: OptionsType;
  name: string;
  label?: string;
  placeholder?: string;
  defaultValue?: OptionItem | OptionItem[];
  type?: "single" | "select_multi" | "select_paging";
  paginate?: PaginateConfig;
  props?: any;
  onChange: any;
  isClear: boolean;
};

function Select(props: SelectType) {
  const { type } = props;
  if (type === "select_paging") {
    const {
      name,
      label,
      placeholder,
      defaultValue,
      props: styleProps,
      onChange,
      isClear,
      paginate,
    } = props;
    return (
      <SelectPaging
        name={name}
        label={label}
        placeholder={placeholder}
        defaultValue={
          Array.isArray(defaultValue) ? undefined : (defaultValue as OptionItem)
        }
        props={styleProps}
        onChange={onChange}
        isClear={isClear}
        paginate={paginate as PaginateConfig}
      />
    );
  }
  const {
    options,
    name,
    label,
    placeholder,
    defaultValue,
    props: styleProps,
    onChange,
    isClear,
  } = props;
  return (
    <SelectBase
      options={options}
      name={name}
      label={label}
      placeholder={placeholder}
      defaultValue={defaultValue}
      type={type}
      props={styleProps}
      onChange={onChange}
      isClear={isClear}
    />
  );
}

export default Select;
