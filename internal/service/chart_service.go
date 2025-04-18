package service

import (
	"cursovay/internal/model"
	"os"

	"github.com/wcharczuk/go-chart/v2"
)

func GenerateBarChart(data []model.Manufacturer, column string) error {
	var values []chart.Value
	for _, m := range data {
		var value float64
		switch column {
		case "revenue":
			value = m.Revenue
		case "employees":
			value = float64(m.Employees)
		}
		values = append(values, chart.Value{
			Label: m.Name,
			Value: value,
		})
	}

	graph := chart.BarChart{
		Title: "Manufacturers by " + column,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 40,
			},
		},
		Bars: values,
	}

	f, _ := os.Create("chart.png")
	defer f.Close()
	return graph.Render(chart.PNG, f)
}
