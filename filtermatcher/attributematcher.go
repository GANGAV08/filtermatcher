package filtermatcher

import (
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/GANGAV08/filter_config/filterconfig"
	"github.com/GANGAV08/filterhelper/filterhelper"
	"github.com/GANGAV08/filterset/filterset"
)

type AttributesMatcher []AttributeMatcher

// AttributeMatcher is a attribute key/value pair to match to.
type AttributeMatcher struct {
	Key string
	// If both AttributeValue and StringFilter are nil only check for key existence.
	AttributeValue *pdata.AttributeValue
	// StringFilter is needed to match against a regular expression
	StringFilter filterset.FilterSet
}

var errUnexpectedAttributeType = errors.New("unexpected attribute type")

func NewAttributesMatcher(config filterset.Config, attributes []filterconfig.Attribute) (AttributesMatcher, error) {
	// Convert attribute values from mp representation to in-memory representation.
	var rawAttributes []AttributeMatcher
	for _, attribute := range attributes {

		if attribute.Key == "" {
			return nil, errors.New("can't have empty key in the list of attributes")
		}

		entry := AttributeMatcher{
			Key: attribute.Key,
		}
		if attribute.Value != nil {
			val, err := filterhelper.NewAttributeValueRaw(attribute.Value)
			if err != nil {
				return nil, err
			}

			if config.MatchType == filterset.Regexp {
				if val.Type() != pdata.AttributeValueTypeString {
					return nil, fmt.Errorf(
						"%s=%s for %q only supports STRING, but found %s",
						filterset.MatchTypeFieldName, filterset.Regexp, attribute.Key, val.Type(),
					)
				}

				filter, err := filterset.CreateFilterSet([]string{val.StringVal()}, &config)
				if err != nil {
					return nil, err
				}
				entry.StringFilter = filter
			} else {
				entry.AttributeValue = &val
			}
		}

		rawAttributes = append(rawAttributes, entry)
	}
	return rawAttributes, nil
}

// Match attributes specification against a span/log.
func (ma AttributesMatcher) Match(attrs pdata.AttributeMap) bool {
	// If there are no attributes to match against, the span/log matches.
	if len(ma) == 0 {
		return true
	}

	// At this point, it is expected of the span/log to have attributes because of
	// len(ma) != 0. This means for spans/logs with no attributes, it does not match.
	if attrs.Len() == 0 {
		return false
	}

	// Check that all expected properties are set.
	for _, property := range ma {
		attr, exist := attrs.Get(property.Key)
		if !exist {
			return false
		}

		if property.StringFilter != nil {
			value, err := attributeStringValue(attr)
			if err != nil || !property.StringFilter.Matches(value) {
				return false
			}
		} else if property.AttributeValue != nil {
			if !attr.Equal(*property.AttributeValue) {
				return false
			}
		}
	}
	return true
}

func attributeStringValue(attr pdata.AttributeValue) (string, error) {
	switch attr.Type() {
	case pdata.AttributeValueTypeString:
		return attr.StringVal(), nil
	case pdata.AttributeValueTypeBool:
		return strconv.FormatBool(attr.BoolVal()), nil
	case pdata.AttributeValueTypeDouble:
		return strconv.FormatFloat(attr.DoubleVal(), 'f', -1, 64), nil
	case pdata.AttributeValueTypeInt:
		return strconv.FormatInt(attr.IntVal(), 10), nil
	default:
		return "", errUnexpectedAttributeType
	}
}
