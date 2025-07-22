package echoApi

import (
	"reflect"
	"strconv"
	"strings"
)

func buildDefaultData(t reflect.Type) map[string]*reflect.Value {
	defaultData := make(map[string]*reflect.Value)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("buildDefaultData: need a struct type")
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 忽略私有字段
		if !field.IsExported() {
			continue
		}

		// 匿名字段，递归处理
		if field.Anonymous {
			subData := buildDefaultData(field.Type)
			for k, v := range subData {
				defaultData[k] = v
			}
			continue
		}

		// 取json标签
		key := field.Name
		if tagName := field.Tag.Get("json"); tagName != "" && tagName != "-" {
			key = strings.Split(tagName, ",")[0]
		}

		// 取 default 标签
		def := field.Tag.Get("default")

		if def != "" {
			// 普通字段：有 default 标签的直接处理
			val := parseDefaultValue(field.Type, def)
			defaultData[key] = &val
		} else {
			// 结构体类型，需要递归去设置
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				// 递归处理子结构体
				subData := buildDefaultData(ft)
				for subKey, subVal := range subData {
					// key 叠加，比如 Parent.Name
					fullKey := key + "." + subKey
					defaultData[fullKey] = subVal
				}
			}
		}
	}

	return defaultData
}

func parseDefaultValue(t reflect.Type, def string) reflect.Value {
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(def)

	case reflect.Bool:
		val, _ := strconv.ParseBool(def)
		return reflect.ValueOf(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, _ := strconv.ParseInt(def, 10, 64)
		return reflect.ValueOf(reflect.ValueOf(val).Convert(t).Interface())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, _ := strconv.ParseUint(def, 10, 64)
		return reflect.ValueOf(reflect.ValueOf(val).Convert(t).Interface())

	case reflect.Float32, reflect.Float64:
		val, _ := strconv.ParseFloat(def, 64)
		return reflect.ValueOf(reflect.ValueOf(val).Convert(t).Interface())

	case reflect.Slice:
		// 切片默认值，比如 default:"a,b,c" 或 default:"1,2,3"
		elemType := t.Elem()
		strs := splitComma(def)
		slice := reflect.MakeSlice(t, len(strs), len(strs))
		for i, s := range strs {
			elem := parseDefaultValue(elemType, s)
			slice.Index(i).Set(elem)
		}
		return slice

	case reflect.Map:
		// map 默认值，比如 default:"k1:v1,k2:v2"
		keyType := t.Key()
		valType := t.Elem()
		m := reflect.MakeMap(t)

		pairs := splitComma(def)
		for _, pair := range pairs {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				continue
			}
			k := parseDefaultValue(keyType, kv[0])
			v := parseDefaultValue(valType, kv[1])
			m.SetMapIndex(k, v)
		}
		return m

	case reflect.Ptr:
		// 指针，递归处理元素
		elem := parseDefaultValue(t.Elem(), def)
		ptr := reflect.New(t.Elem())
		ptr.Elem().Set(elem)
		return ptr

	default:
		// 不支持的类型，返回零值
		return reflect.Zero(t)
	}
}

func applyDefaults(ptr interface{}, defaultData map[string]*reflect.Value) {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		panic("applyDefaults: need a non-nil pointer")
	}
	v = v.Elem()

	if v.Kind() != reflect.Struct {
		panic("applyDefaults: need a struct pointer")
	}

	for key, defVal := range defaultData {
		path := strings.Split(key, ".")
		setFieldValue(v, path, defVal)
	}
}
func setFieldValue(v reflect.Value, path []string, defVal *reflect.Value) {
	if len(path) == 0 {
		return
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	field := v.FieldByName(path[0])
	if !field.IsValid() || !field.CanSet() {
		return
	}

	if len(path) == 1 {
		// 最后一级，设置值
		if isZero(field) {
			field.Set(defVal.Convert(field.Type()))
		}
		return
	}

	// 中间节点，递归
	setFieldValue(field, path[1:], defVal)
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return v.IsNil()
	case reflect.Array:
		// 数组类型，需要检查每一项
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		// 结构体类型，需要检查每一个字段
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			res = append(res, part)
		}
	}
	return res
}
