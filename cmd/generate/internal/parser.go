package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// SchemaParser handles parsing JSON Schema files to extract type definitions.
type SchemaParser struct {
	schema map[string]interface{}
	defs   map[string]interface{}
}

// NewSchemaParser creates a new parser from a schema file path.
func NewSchemaParser(schemaPath string) (*SchemaParser, error) {
	file, err := os.Open(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("opening schema file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}

	var schema map[string]interface{}
	if unmarshalErr := json.Unmarshal(data, &schema); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", unmarshalErr)
	}

	defs, ok := schema["$defs"].(map[string]interface{})
	if !ok {
		return nil, errors.New("schema missing $defs section")
	}

	return &SchemaParser{
		schema: schema,
		defs:   defs,
	}, nil
}

// EnumDefinition represents a string enum from the schema.
type EnumDefinition struct {
	Name         string
	Description  string
	Values       []EnumValue
	DefaultValue string
}

// EnumValue represents a single enum value with its description.
type EnumValue struct {
	Value       string
	Description string
}

// UnionDefinition represents a oneOf discriminated union.
type UnionDefinition struct {
	Name          string
	Description   string
	Discriminator string
	Variants      []UnionVariant
}

// UnionVariant represents one variant in a discriminated union.
type UnionVariant struct {
	Name             string
	Value            string
	Description      string
	Properties       map[string]interface{}
	SortedProperties []PropertyPair
	Required         []string
}

// PropertyPair represents a property name/definition pair for ordered iteration.
type PropertyPair struct {
	Name string
	Def  interface{}
}

// FindEnums identifies all string enums (oneOf with const values).
func (p *SchemaParser) FindEnums() ([]EnumDefinition, error) {
	var enums []EnumDefinition

	for name, defInterface := range p.defs {
		def, ok := defInterface.(map[string]interface{})
		if !ok {
			continue
		}

		oneOf, hasOneOf := def["oneOf"]
		if !hasOneOf {
			continue
		}

		oneOfArray, ok := oneOf.([]interface{})
		if !ok {
			continue
		}

		// Check if this is a string enum (all variants have const + type: string)
		if isStringEnum(oneOfArray) {
			enum := EnumDefinition{
				Name:        name,
				Description: getDescription(def),
				Values:      make([]EnumValue, 0, len(oneOfArray)),
			}

			for _, variantInterface := range oneOfArray {
				enumVariant, enumOk := variantInterface.(map[string]interface{})
				if !enumOk {
					continue
				}

				constValue, hasConst := enumVariant["const"]
				if !hasConst {
					continue
				}

				value, valueOk := constValue.(string)
				if !valueOk {
					continue
				}

				enum.Values = append(enum.Values, EnumValue{
					Value:       value,
					Description: getDescription(enumVariant),
				})
			}

			// Sort values for consistent generation
			sort.Slice(enum.Values, func(i, j int) bool {
				return enum.Values[i].Value < enum.Values[j].Value
			})

			enums = append(enums, enum)
		}
	}

	// Sort enums by name for consistent generation
	sort.Slice(enums, func(i, j int) bool {
		return enums[i].Name < enums[j].Name
	})

	return enums, nil
}

// FindUnions identifies discriminated unions (oneOf with object variants).
//
//nolint:gocognit,nestif // Complex schema parsing requires high cognitive complexity and nested conditions
func (p *SchemaParser) FindUnions() ([]UnionDefinition, error) {
	var unions []UnionDefinition

	for name, defInterface := range p.defs {
		def, ok := defInterface.(map[string]interface{})
		if !ok {
			continue
		}

		oneOf, hasOneOf := def["oneOf"]
		if !hasOneOf {
			continue
		}

		oneOfArray, ok := oneOf.([]interface{})
		if !ok {
			continue
		}

		// Check if this is a discriminated union (all variants are objects with discriminator)
		discriminator := findDiscriminator(oneOfArray)
		if discriminator != "" {
			union := UnionDefinition{
				Name:          name,
				Description:   getDescription(def),
				Discriminator: discriminator,
				Variants:      make([]UnionVariant, 0, len(oneOfArray)),
			}

			for _, variantInterface := range oneOfArray {
				unionVariant, unionOk := variantInterface.(map[string]interface{})
				if !unionOk {
					continue
				}

				properties, hasProps := unionVariant["properties"].(map[string]interface{})
				if !hasProps {
					continue
				}

				// Get discriminator value
				discProp, hasDiscProp := properties[discriminator].(map[string]interface{})
				if !hasDiscProp {
					continue
				}

				discValue, hasDiscValue := discProp["const"].(string)
				if !hasDiscValue {
					continue
				}

				// Get required fields
				required := make([]string, 0)
				if reqInterface, hasReq := unionVariant["required"]; hasReq {
					if reqArray, reqOk := reqInterface.([]interface{}); reqOk {
						for _, r := range reqArray {
							if reqStr, reqStrOk := r.(string); reqStrOk {
								required = append(required, reqStr)
							}
						}
					}
				}

				// Create sorted properties for consistent parameter generation
				var sortedProps []PropertyPair
				propNames := make([]string, 0, len(properties))
				for propName := range properties {
					propNames = append(propNames, propName)
				}
				sort.Strings(propNames)
				for _, propName := range propNames {
					sortedProps = append(sortedProps, PropertyPair{
						Name: propName,
						Def:  properties[propName],
					})
				}

				union.Variants = append(union.Variants, UnionVariant{
					Name:             toCamelCase(discValue),
					Value:            discValue,
					Description:      getDescription(unionVariant),
					Properties:       properties,
					SortedProperties: sortedProps,
					Required:         required,
				})
			}

			// Sort variants by value for consistent generation
			sort.Slice(union.Variants, func(i, j int) bool {
				return union.Variants[i].Value < union.Variants[j].Value
			})

			unions = append(unions, union)
		}
	}

	// Sort unions by name for consistent generation
	sort.Slice(unions, func(i, j int) bool {
		return unions[i].Name < unions[j].Name
	})

	return unions, nil
}

// isStringEnum checks if a oneOf array represents a string enum.
func isStringEnum(oneOfArray []interface{}) bool {
	if len(oneOfArray) == 0 {
		return false
	}

	for _, variantInterface := range oneOfArray {
		variant, ok := variantInterface.(map[string]interface{})
		if !ok {
			return false
		}

		// Must have const value
		_, hasConst := variant["const"]
		if !hasConst {
			return false
		}

		// Must have type: string
		variantType, hasType := variant["type"].(string)
		if !hasType || variantType != "string" {
			return false
		}
	}

	return true
}

// findDiscriminator finds the common discriminator field in oneOf variants.
func findDiscriminator(oneOfArray []interface{}) string {
	if len(oneOfArray) == 0 {
		return ""
	}

	// Check common discriminator patterns
	commonFields := []string{"type", "sessionUpdate", "kind"}

	for _, field := range commonFields {
		if hasDiscriminatorField(oneOfArray, field) {
			return field
		}
	}

	return ""
}

// hasDiscriminatorField checks if all variants have a const value for the given field.
func hasDiscriminatorField(oneOfArray []interface{}, field string) bool {
	for _, variantInterface := range oneOfArray {
		variant, ok := variantInterface.(map[string]interface{})
		if !ok {
			return false
		}

		properties, hasProps := variant["properties"].(map[string]interface{})
		if !hasProps {
			return false
		}

		fieldProp, hasField := properties[field].(map[string]interface{})
		if !hasField {
			return false
		}

		_, hasConst := fieldProp["const"]
		if !hasConst {
			return false
		}
	}

	return true
}

// getDescription extracts description from a schema object.
func getDescription(obj map[string]interface{}) string {
	if desc, ok := obj["description"].(string); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

// toCamelCase converts snake_case or kebab-case to CamelCase.
func toCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(c rune) bool {
		return c == '_' || c == '-' || c == '/'
	})

	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return result
}
