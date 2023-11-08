package common

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

func GenerateSolidityStruct(goStruct any) (string, error) {
	structCodeRecorder := map[reflect.Type]string{}
	typ := reflect.TypeOf(goStruct)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	_, err := getStructType(typ, structCodeRecorder)
	if err != nil {
		return "", err
	}
	code := "\n"
	for _, t := range structCodeRecorder {
		code += t + "\n"
	}
	return code, nil
}

func getStructType(structType reflect.Type, structRecorder map[reflect.Type]string) (string, error) {
	if structType.Kind() != reflect.Struct {
		return "", errors.Errorf("only support struct or struct ptr, but get %s", structType.String())
	}

	structName := structType.Name()
	if _, ok := structRecorder[structType]; !ok {
		solidityStruct := fmt.Sprintf("struct %s {\n", structName)

		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			fieldName := field.Name
			fieldType := field.Type
			solidityType, err := goTypeToSolidityType(fieldType, structRecorder)
			if err != nil {
				return "", err
			}
			solidityStruct += fmt.Sprintf("	%s %s;\n", solidityType, fieldName)
		}

		solidityStruct += "}\n"
		structRecorder[structType] = solidityStruct
	}
	return structName, nil
}

func goTypeToSolidityType(goType reflect.Type, structRecorder map[reflect.Type]string) (string, error) {
	switch goType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int64", nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint64", nil
	case reflect.String:
		return "string", nil
	case reflect.Bool:
		return "bool", nil
	case reflect.Map:
		keyType, err := goTypeToSolidityType(goType.Key(), structRecorder)
		if err != nil {
			return "", err
		}
		valueType, err := goTypeToSolidityType(goType.Elem(), structRecorder)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("mapping(%s => %s)", keyType, valueType), nil
	case reflect.Slice:
		elementType, err := goTypeToSolidityType(goType.Elem(), structRecorder)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s[]", elementType), nil
	case reflect.Ptr:
		return goTypeToSolidityType(goType.Elem(), structRecorder)
	case reflect.Struct:
		return getStructType(goType, structRecorder)
	default:
		return "", errors.Errorf("unsupported field type %s", goType.String())
	}
}
