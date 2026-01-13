package condition

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
)

// DeviceSchema contains minimal device schema info needed for condition evaluation
type DeviceSchema struct {
	SwConfig bool
	Version  string
}

// ConditionContext holds the context for evaluating conditions
type ConditionContext struct {
	DeviceConfig *config.DeviceConfig
	DeviceSchema *DeviceSchema
}

// Evaluate evaluates a condition string and returns true if it matches
func Evaluate(condition *string, ctx *ConditionContext) bool {
	if condition == nil || *condition == "*" {
		return true
	}

	// Build the LHS mapping
	lhsMapping := buildLHSMapping(ctx)

	// Parse and evaluate the condition
	return evaluateExpression(*condition, lhsMapping)
}

func buildLHSMapping(ctx *ConditionContext) map[string]interface{} {
	mapping := make(map[string]interface{})

	// Add device properties
	mapping["device.sw_config"] = ctx.DeviceSchema.SwConfig
	mapping["device.hostname"] = ctx.DeviceConfig.Hostname
	mapping["device.ipaddr"] = ctx.DeviceConfig.IPAddr
	mapping["device.model_id"] = ctx.DeviceConfig.ModelID
	mapping["device.version"] = ctx.DeviceSchema.Version

	// Add device tags
	for tagKey, tagValue := range ctx.DeviceConfig.Tags {
		mapping[fmt.Sprintf("device.tag.%s", tagKey)] = tagValue
	}

	return mapping
}

func evaluateExpression(expr string, lhsMapping map[string]interface{}) bool {
	// Split by OR (||)
	orParts := splitByOperator(expr, "||")

	for _, orPart := range orParts {
		// Split by AND (&&)
		andParts := splitByOperator(orPart, "&&")

		allTrue := true
		for _, andPart := range andParts {
			if !evaluateComparison(strings.TrimSpace(andPart), lhsMapping) {
				allTrue = false
				break
			}
		}

		if allTrue {
			return true
		}
	}

	return false
}

func splitByOperator(expr string, operator string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	i := 0
	for i < len(expr) {
		if expr[i] == '\'' || expr[i] == '"' {
			inQuotes = !inQuotes
			current.WriteByte(expr[i])
			i++
			continue
		}

		if !inQuotes && i+len(operator) <= len(expr) && expr[i:i+len(operator)] == operator {
			parts = append(parts, current.String())
			current.Reset()
			i += len(operator)
			continue
		}

		current.WriteByte(expr[i])
		i++
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func evaluateComparison(expr string, lhsMapping map[string]interface{}) bool {
	expr = strings.TrimSpace(expr)

	// Try to split by ==
	if parts := splitComparison(expr, "=="); len(parts) == 2 {
		lhs := strings.TrimSpace(parts[0])
		rhs := strings.TrimSpace(parts[1])

		lhsValue, ok := lhsMapping[lhs]
		if !ok {
			panic(fmt.Sprintf("Invalid conditional parameter: %s", lhs))
		}

		rhsValue := parseValue(rhs)
		return compareValues(lhsValue, rhsValue, true)
	}

	// Try to split by !=
	if parts := splitComparison(expr, "!="); len(parts) == 2 {
		lhs := strings.TrimSpace(parts[0])
		rhs := strings.TrimSpace(parts[1])

		lhsValue, ok := lhsMapping[lhs]
		if !ok {
			panic(fmt.Sprintf("Invalid conditional parameter: %s", lhs))
		}

		rhsValue := parseValue(rhs)
		return compareValues(lhsValue, rhsValue, false)
	}

	panic(fmt.Sprintf("Unable to parse condition: %s", expr))
}

func splitComparison(expr string, operator string) []string {
	// Find the operator, avoiding it inside quotes
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(expr); i++ {
		if expr[i] == '\'' || expr[i] == '"' {
			if !inQuotes {
				inQuotes = true
				quoteChar = expr[i]
			} else if expr[i] == quoteChar {
				inQuotes = false
			}
			continue
		}

		if !inQuotes && i+len(operator) <= len(expr) && expr[i:i+len(operator)] == operator {
			return []string{expr[:i], expr[i+len(operator):]}
		}
	}

	return []string{expr}
}

func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Remove quotes
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		s = s[1 : len(s)-1]
	}

	// Try to parse as JSON for booleans, numbers, etc.
	var value interface{}
	if err := json.Unmarshal([]byte(s), &value); err == nil {
		return value
	}

	// Otherwise return as string
	return s
}

func compareValues(lhs, rhs interface{}, equals bool) bool {
	// Handle array values (tag can be an array)
	if arr, ok := lhs.([]interface{}); ok {
		contains := false
		for _, item := range arr {
			if compareScalar(item, rhs) {
				contains = true
				break
			}
		}
		if equals {
			return contains
		}
		return !contains
	}

	// Handle map values (tags can be strings or arrays)
	if arr, ok := lhs.([]string); ok {
		contains := false
		for _, item := range arr {
			if compareScalar(item, rhs) {
				contains = true
				break
			}
		}
		if equals {
			return contains
		}
		return !contains
	}

	// Scalar comparison
	result := compareScalar(lhs, rhs)
	if equals {
		return result
	}
	return !result
}

func compareScalar(lhs, rhs interface{}) bool {
	// Handle boolean comparison
	if lhsBool, ok := lhs.(bool); ok {
		if rhsBool, ok := rhs.(bool); ok {
			return lhsBool == rhsBool
		}
	}

	// Handle string comparison
	lhsStr := fmt.Sprintf("%v", lhs)
	rhsStr := fmt.Sprintf("%v", rhs)

	return lhsStr == rhsStr
}
