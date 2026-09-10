package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type point struct {
	peers     int
	mean, std float64
}

var series = []struct{ key, label, color string }{
	{"gossipsub", "Standard GossipSub", "#4C78A8"},
	{"fedgreen", "FedGreenSub", "#F58518"},
	{"trustaware", "Trust-Aware FedGreenSub", "#54A24B"},
}

func main() {
	root := filepath.Join("evaluation", "comparison")
	rows, err := readRows(filepath.Join(root, "results", "energy_scaling", "energy_scaling_summary.csv"))
	if err != nil {
		panic(err)
	}
	out := filepath.Join(root, "plots", "output", "energy_scaling")
	if err := os.MkdirAll(out, 0755); err != nil {
		panic(err)
	}
	writeSVG(filepath.Join(out, "energy_cost_vs_peers.svg"), "Modeled resource cost vs number of peers", "Modeled resource-cost units per workload", rows, "modeled_resource_cost_total_mean", "modeled_resource_cost_total_stddev")
	writeSVG(filepath.Join(out, "energy_cost_per_delivered_vs_peers.svg"), "Modeled cost per delivered message vs number of peers", "Modeled cost per delivered message", rows, "modeled_resource_cost_per_delivered_mean", "modeled_resource_cost_per_delivered_stddev")
	writeSVG(filepath.Join(out, "energy_reduction_vs_peers.svg"), "Modeled resource-cost reduction vs Standard GossipSub", "Reduction vs baseline (%)", rows, "reduction_from_baseline_percent", "")
	fmt.Printf("Generated SVG plots under %s\n", out)
}

func readRows(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty summary")
	}
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(records[0]))
		for i, key := range records[0] {
			if i < len(record) {
				row[key] = record[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func points(rows []map[string]string, implementation, field, stdField string) []point {
	out := make([]point, 0)
	for _, row := range rows {
		if row["implementation"] != implementation {
			continue
		}
		peers, _ := strconv.Atoi(row["peer_count"])
		mean, _ := strconv.ParseFloat(row[field], 64)
		std := 0.0
		if stdField != "" {
			std, _ = strconv.ParseFloat(row[stdField], 64)
		}
		out = append(out, point{peers, mean, std})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].peers < out[j].peers })
	return out
}

func writeSVG(path, title, ylabel string, rows []map[string]string, field, stdField string) {
	const width, height, left, right, top, bottom = 900, 560, 90, 30, 70, 80
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, s := range series {
		for _, p := range points(rows, s.key, field, stdField) {
			x := float64(p.peers)
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if p.mean-p.std < minY {
				minY = p.mean - p.std
			}
			if p.mean+p.std > maxY {
				maxY = p.mean + p.std
			}
		}
	}
	if field == "reduction_from_baseline_percent" {
		minY = math.Min(minY, 0)
		maxY = math.Max(maxY, 0)
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}
	pad := (maxY - minY) * .08
	minY -= pad
	maxY += pad
	x := func(v float64) float64 { return left + (v-minX)/math.Max(maxX-minX, 1)*float64(width-left-right) }
	y := func(v float64) float64 { return top + (maxY-v)/math.Max(maxY-minY, 1)*float64(height-top-bottom) }
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	fmt.Fprintf(f, "<rect width=\"100%%\" height=\"100%%\" fill=\"white\"/><text x=\"%d\" y=\"34\" text-anchor=\"middle\" font-family=\"Arial\" font-size=\"20\">%s</text>\n", width/2, title)
	for i := 0; i <= 5; i++ {
		value := minY + (maxY-minY)*float64(i)/5
		py := y(value)
		fmt.Fprintf(f, "<line x1=\"%g\" y1=\"%g\" x2=\"%g\" y2=\"%g\" stroke=\"#dddddd\"/><text x=\"%g\" y=\"%g\" text-anchor=\"end\" font-family=\"Arial\" font-size=\"12\">%.4g</text>\n", float64(left), py, float64(width-right), py, float64(left-8), py+4, value)
	}
	for _, s := range series {
		ps := points(rows, s.key, field, stdField)
		if len(ps) == 0 {
			continue
		}
		fmt.Fprintf(f, "<polyline fill=\"none\" stroke=\"%s\" stroke-width=\"3\" points=\"", s.color)
		for _, p := range ps {
			fmt.Fprintf(f, "%g,%g ", x(float64(p.peers)), y(p.mean))
		}
		fmt.Fprintln(f, "\"/>")
		for _, p := range ps {
			px, py := x(float64(p.peers)), y(p.mean)
			fmt.Fprintf(f, "<line x1=\"%g\" y1=\"%g\" x2=\"%g\" y2=\"%g\" stroke=\"%s\"/><circle cx=\"%g\" cy=\"%g\" r=\"5\" fill=\"%s\"/>\n", px, y(p.mean-p.std), px, y(p.mean+p.std), s.color, px, py, s.color)
		}
	}
	for _, p := range points(rows, "gossipsub", field, stdField) {
		fmt.Fprintf(f, "<text x=\"%g\" y=\"%d\" text-anchor=\"middle\" font-family=\"Arial\" font-size=\"12\">%d</text>\n", x(float64(p.peers)), height-bottom+22, p.peers)
	}
	fmt.Fprintf(f, "<text x=\"%d\" y=\"%d\" text-anchor=\"middle\" font-family=\"Arial\" font-size=\"14\">Number of peers</text><text transform=\"translate(20 %d) rotate(-90)\" text-anchor=\"middle\" font-family=\"Arial\" font-size=\"14\">%s</text>\n", width/2, height-22, height/2, ylabel)
	for i, s := range series {
		lx := width - 270
		ly := 90 + i*24
		fmt.Fprintf(f, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"%s\" stroke-width=\"3\"/><text x=\"%d\" y=\"%d\" font-family=\"Arial\" font-size=\"12\">%s</text>\n", lx, ly, lx+24, ly, s.color, lx+32, ly+4, s.label)
	}
	fprintln(f, "</svg>")
}

func fprintln(f *os.File, value string) { _, _ = fmt.Fprintln(f, value) }
