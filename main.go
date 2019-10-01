package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/go-openapi/swag"
	"github.com/iancoleman/orderedmap"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

const xOrder = "x-order" // sort order for properties (or any schema)

// OrderSpec is a command that flattens a swagger document
// which will expand the remote references in a spec and move inline schemas to definitions
// after flattening there are no complex inlined anymore
type OrderSpec struct {
	Compact bool
	Output  string
	Format  string
}

func main() {
	c := OrderSpec{}
	kingpin.CommandLine.Name = "swagger-order"

	kingpin.Flag("compact", "applies to JSON formatted specs. When present, doesn't prettify the json").BoolVar(&c.Compact)
	kingpin.Flag("output", "the file to write to").Short('o').StringVar(&c.Output)
	formats := []string{"json", "yaml"}
	formatsHelp := fmt.Sprintf("the format for the spec document, one of: %s (default: %s)", strings.Join(formats, ", "), formats[0])
	kingpin.Flag("format", formatsHelp).Default(formats[0]).EnumVar(&c.Format, formats...)
	swaggerDoc := kingpin.Arg("swagger-doc", "swagger document url").String()

	kingpin.Parse()

	err := c.Execute(*swaggerDoc)
	if err != nil {
		log.Fatal(err)
	}
}

// Execute flattens the spec
func (c *OrderSpec) Execute(swaggerDoc string) error {

	doc, err := OrderByXOrder(swaggerDoc)
	if err != nil {
		return err
	}

	return writeOrderedSpecToFile(doc, !c.Compact, c.Format, string(c.Output))
}

func OrderByXOrder(specPath string) (*orderedmap.OrderedMap, error) {
	var convertToOrderedOutput func(ele interface{}) *orderedmap.OrderedMap
	convertToOrderedOutput = func(ele interface{}) *orderedmap.OrderedMap {
		o := orderedmap.New()
		if slice, ok := ele.(yaml.MapSlice); ok {
			for _, v := range slice {
				if slice, ok := v.Value.(yaml.MapSlice); ok {
					o.Set(v.Key.(string), convertToOrderedOutput(slice))
				} else if items, ok := v.Value.([]interface{}); ok {
					newItems := []interface{}{}
					for _, item := range items {
						if slice, ok := item.(yaml.MapSlice); ok {
							newItems = append(newItems, convertToOrderedOutput(slice))
						} else {
							newItems = append(newItems, item)
						}
					}
					o.Set(v.Key.(string), newItems)
				} else {
					o.Set(v.Key.(string), v.Value)
				}
			}
		}
		o.Sort(func(a *orderedmap.Pair, b *orderedmap.Pair) bool {
			return getXOrder(a.Value()) < getXOrder(b.Value())
		})
		return o
	}

	yamlDoc, err := swag.YAMLData(specPath)
	if err != nil {
		panic(err)
	}

	return convertToOrderedOutput(yamlDoc), nil
}

func getXOrder(val interface{}) int {
	if prop, ok := val.(*orderedmap.OrderedMap); ok {
		if pSlice, ok := prop.Get(xOrder); ok {
			return pSlice.(int)
		}
	}
	return 0
}

func writeOrderedSpecToFile(swspec *orderedmap.OrderedMap, pretty bool, format string, output string) error {
	var b []byte
	var err error
	asJSON := format == "json"

	if pretty && asJSON {
		b, err = json.MarshalIndent(swspec, "", "  ")
	} else if asJSON {
		b, err = json.Marshal(swspec)
	} else {
		// marshals as YAML
		b, err = json.Marshal(swspec)
		if err == nil {
			d, ery := swag.BytesToYAMLDoc(b)
			if ery != nil {
				return ery
			}
			b, err = yaml.Marshal(d)
		}
	}
	if err != nil {
		return err
	}
	if output == "" {
		fmt.Println(string(b))
		return nil
	}
	return ioutil.WriteFile(output, b, 0644)
}
