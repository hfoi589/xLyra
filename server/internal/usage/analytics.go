package usage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

const (
	AnalyticsViewBar     = "bar"
	AnalyticsViewScatter = "scatter"
	AnalyticsViewSankey  = "sankey"

	analyticsBucketHour = "hour"
	analyticsBucketDay  = "day"
	analyticsBucketWeek = "week"
	analyticsBucket15m  = "15min"

	analyticsTopModels  = 12
	analyticsTopKeys    = 8
	analyticsTopNodes   = 12
	analyticsMaxScatter = 5000
)

var (
	ErrInvalidAnalyticsView     = errors.New("invalid analytics view")
	ErrInvalidAnalyticsRange    = errors.New("invalid analytics range")
	ErrAnalyticsRangeTooLarge   = errors.New("analytics range exceeds 365 days")
	ErrAnalyticsCurrencyMissing = errors.New("analytics currency is not available")
)

type AnalyticsQuery struct {
	View        string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	ModelKeys   []string
	SiteIDs     []uuid.UUID
	APIKeyIDs   []uuid.UUID
	Success     *bool
	AllStatuses bool
	Currency    string
}

type AnalyticsResponse struct {
	Meta AnalyticsMeta `json:"meta"`
	Data any           `json:"data"`
}

type AnalyticsMeta struct {
	View                string   `json:"view"`
	TimeZone            string   `json:"timezone"`
	RangeStart          string   `json:"range_start"`
	RangeEnd            string   `json:"range_end"`
	BucketUnit          string   `json:"bucket_unit"`
	Currency            string   `json:"currency"`
	AvailableCurrencies []string `json:"available_currencies"`
	Truncated           bool     `json:"truncated"`
	TooManyPoints       bool     `json:"too_many_points"`
	TotalRequests       int64    `json:"total_requests"`
	ReturnedPoints      int      `json:"returned_points"`
	MissingCostRequests int64    `json:"missing_cost_requests"`
	SuggestedAction     string   `json:"suggested_action,omitempty"`
}

type AnalyticsBarData struct {
	BucketUnit string               `json:"bucket_unit"`
	Series     []AnalyticsBarSeries `json:"series"`
	Points     []AnalyticsBarPoint  `json:"points"`
}

type AnalyticsBarSeries struct {
	ModelKey    string `json:"model_key"`
	ModelLabel  string `json:"model_label"`
	APIKeyID    string `json:"api_key_id"`
	APIKeyLabel string `json:"api_key_label"`
	IsOther     bool   `json:"is_other"`
}

type AnalyticsBarPoint struct {
	BucketStart string              `json:"bucket_start"`
	BucketEnd   string              `json:"bucket_end"`
	Groups      []AnalyticsBarGroup `json:"groups"`
}

type AnalyticsBarGroup struct {
	ModelKey   string                `json:"model_key"`
	ModelLabel string                `json:"model_label"`
	TotalCost  float64               `json:"total_cost"`
	Segments   []AnalyticsBarSegment `json:"segments"`
}

type AnalyticsBarSegment struct {
	APIKeyID    string  `json:"api_key_id"`
	APIKeyLabel string  `json:"api_key_label"`
	Cost        float64 `json:"cost"`
	IsOther     bool    `json:"is_other"`
}

type analyticsBarBuildResult struct {
	AnalyticsBarData
	TotalCost           float64 `json:"-"`
	MissingCostRequests int64   `json:"-"`
}

type AnalyticsScatterData struct {
	Points []AnalyticsScatterPoint `json:"points"`
}

type AnalyticsScatterPoint struct {
	BucketStart  string  `json:"bucket_start"`
	BucketEnd    string  `json:"bucket_end"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	Currency     string  `json:"currency"`
}

type AnalyticsSankeyData struct {
	Nodes       []AnalyticsSankeyNode `json:"nodes"`
	Links       []AnalyticsSankeyLink `json:"links"`
	TotalTokens int64                 `json:"total_tokens"`
}

type AnalyticsSankeyNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Type    string `json:"type"`
	IsOther bool   `json:"is_other"`
}

type AnalyticsSankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

type analyticsNormalizedQuery struct {
	AnalyticsQuery
	From time.Time
	To   time.Time
}

func (s *Service) normalizeAnalyticsQuery(input AnalyticsQuery, now time.Time) (analyticsNormalizedQuery, error) {
	if s == nil {
		return analyticsNormalizedQuery{}, fmt.Errorf("usage service is not initialized")
	}
	view := strings.ToLower(strings.TrimSpace(input.View))
	if view == "" {
		view = AnalyticsViewBar
	}
	if view != AnalyticsViewBar && view != AnalyticsViewScatter && view != AnalyticsViewSankey {
		return analyticsNormalizedQuery{}, ErrInvalidAnalyticsView
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = s.timeZone.In(now)
	from := now.Add(-7 * 24 * time.Hour)
	to := now
	if input.CreatedFrom != nil {
		from = s.timeZone.In(*input.CreatedFrom)
	}
	if input.CreatedTo != nil {
		to = s.timeZone.In(*input.CreatedTo)
	}
	if !to.After(from) {
		return analyticsNormalizedQuery{}, ErrInvalidAnalyticsRange
	}
	if to.Sub(from) > 365*24*time.Hour {
		return analyticsNormalizedQuery{}, ErrAnalyticsRangeTooLarge
	}
	success := true
	var successFilter *bool = &success
	if view != AnalyticsViewBar {
		switch {
		case input.AllStatuses:
			successFilter = nil
		case input.Success != nil:
			success = *input.Success
		}
	}
	models := make([]string, 0, len(input.ModelKeys))
	seenModels := map[string]struct{}{}
	for _, model := range input.ModelKeys {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seenModels[model]; ok {
			continue
		}
		seenModels[model] = struct{}{}
		models = append(models, model)
	}
	return analyticsNormalizedQuery{
		AnalyticsQuery: AnalyticsQuery{
			View:        view,
			CreatedFrom: timePtr(from),
			CreatedTo:   timePtr(to),
			ModelKeys:   models,
			SiteIDs:     uniqueAnalyticsUUIDs(input.SiteIDs),
			APIKeyIDs:   uniqueAnalyticsUUIDs(input.APIKeyIDs),
			Success:     successFilter,
			Currency:    strings.ToUpper(strings.TrimSpace(input.Currency)),
		},
		From: from,
		To:   to,
	}, nil
}

func (s *Service) Analytics(ctx context.Context, input AnalyticsQuery, now time.Time) (AnalyticsResponse, error) {
	if s == nil || s.db == nil || s.db.DB() == nil {
		return AnalyticsResponse{}, fmt.Errorf("request analytics store is not initialized")
	}
	normalized, err := s.normalizeAnalyticsQuery(input, now)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	repo := store.NewRequestAnalyticsRepository(s.db.DB())
	storeQuery := store.RequestAnalyticsQuery{
		CreatedFrom: normalized.From,
		CreatedTo:   normalized.To,
		ModelKeys:   normalized.ModelKeys,
		SiteIDs:     normalized.SiteIDs,
		APIKeyIDs:   normalized.APIKeyIDs,
		Success:     normalized.Success,
	}
	availableCurrencies, err := repo.ListCurrencies(ctx, storeQuery)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	currency, err := selectAnalyticsCurrency(normalized.Currency, availableCurrencies)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	storeQuery.Currency = currency
	storeQuery.ApplyCurrency = normalized.View != AnalyticsViewSankey
	meta := AnalyticsMeta{
		View:                normalized.View,
		TimeZone:            s.timeZone.Name,
		RangeStart:          normalized.From.Format(time.RFC3339Nano),
		RangeEnd:            normalized.To.Format(time.RFC3339Nano),
		Currency:            currency,
		AvailableCurrencies: availableCurrencies,
	}

	if normalized.View == AnalyticsViewScatter {
		meta.BucketUnit = analyticsBucket15m
		return s.analyticsScatter(ctx, repo, storeQuery, meta)
	}

	_, _, includesSummary := s.requestSummaryBucketRange(&normalized.From, &normalized.To, now)
	bucketUnit := analyticsBucketUnit(normalized.From, normalized.To, includesSummary)
	meta.BucketUnit = bucketUnit
	rows, totalRequests, err := s.analyticsAggregateRows(ctx, repo, storeQuery, normalized, bucketUnit, now)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	meta.TotalRequests = totalRequests
	if normalized.View == AnalyticsViewBar {
		data := buildBarAnalytics(rows, s.timeZone, bucketUnit)
		meta.ReturnedPoints = len(data.Points)
		meta.MissingCostRequests = data.MissingCostRequests
		return AnalyticsResponse{Meta: meta, Data: data}, nil
	}
	data := buildSankeyAnalytics(rows)
	meta.ReturnedPoints = len(data.Links)
	return AnalyticsResponse{Meta: meta, Data: data}, nil
}

func (s *Service) analyticsAggregateRows(ctx context.Context, repo store.RequestAnalyticsRepository, query store.RequestAnalyticsQuery, normalized analyticsNormalizedQuery, bucketUnit string, now time.Time) ([]store.RequestAnalyticsAggregate, int64, error) {
	summaryFrom, summaryTo, includeSummary := s.requestSummaryBucketRange(&normalized.From, &normalized.To, now)
	rows := make([]store.RequestAnalyticsAggregate, 0)
	if includeSummary && summaryFrom != nil && summaryTo != nil && summaryTo.After(*summaryFrom) {
		summaryRows, err := repo.ListSummaryAggregates(ctx, query, *summaryFrom, *summaryTo)
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, summaryRows...)
	}
	for _, window := range s.requestSummaryDetailWindows(&normalized.From, &normalized.To, summaryFrom, summaryTo, now) {
		if !window.To.After(window.From) {
			continue
		}
		detailRows, err := repo.ListDetailAggregates(ctx, query, window.From, window.To, bucketUnit, s.timeZone.Name)
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, detailRows...)
	}
	if bucketUnit == analyticsBucketWeek {
		rows = rebucketAnalyticsRowsByWeek(rows, s.timeZone)
	}
	var totalRequests int64
	for _, row := range rows {
		totalRequests += row.RequestCount
	}
	return rows, totalRequests, nil
}

func (s *Service) analyticsScatter(ctx context.Context, repo store.RequestAnalyticsRepository, query store.RequestAnalyticsQuery, meta AnalyticsMeta) (AnalyticsResponse, error) {
	total, drawable, missing, err := repo.CountScatterBuckets(ctx, query, s.timeZone.Name)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	meta.TotalRequests = total
	meta.MissingCostRequests = missing
	if drawable > analyticsMaxScatter {
		meta.TooManyPoints = true
		meta.SuggestedAction = "narrow_filters"
		return AnalyticsResponse{Meta: meta, Data: AnalyticsScatterData{Points: []AnalyticsScatterPoint{}}}, nil
	}
	rows, err := repo.ListScatterBuckets(ctx, query, s.timeZone.Name)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	data := buildScatterAnalytics(rows)
	meta.ReturnedPoints = len(data.Points)
	return AnalyticsResponse{Meta: meta, Data: data}, nil
}

func buildScatterAnalytics(rows []store.RequestAnalyticsScatterBucket) AnalyticsScatterData {
	points := make([]AnalyticsScatterPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, AnalyticsScatterPoint{
			BucketStart:  row.BucketStart.Format(time.RFC3339Nano),
			BucketEnd:    row.BucketStart.Add(15 * time.Minute).Format(time.RFC3339Nano),
			RequestCount: row.RequestCount,
			TotalTokens:  row.TotalTokens,
			TotalCost:    row.Cost,
			Currency:     row.Currency,
		})
	}
	return AnalyticsScatterData{Points: points}
}

func selectAnalyticsCurrency(requested string, available []string) (string, error) {
	requested = strings.ToUpper(strings.TrimSpace(requested))
	if requested != "" {
		if len(available) == 0 {
			return requested, nil
		}
		for _, value := range available {
			if strings.EqualFold(value, requested) {
				return value, nil
			}
		}
		return "", ErrAnalyticsCurrencyMissing
	}
	for _, value := range available {
		if strings.EqualFold(value, "USD") {
			return value, nil
		}
	}
	if len(available) > 0 {
		return available[0], nil
	}
	return "USD", nil
}

func analyticsBucketUnit(from time.Time, to time.Time, includesSummary bool) string {
	duration := to.Sub(from)
	if duration <= 48*time.Hour && !includesSummary {
		return analyticsBucketHour
	}
	if duration <= 180*24*time.Hour {
		return analyticsBucketDay
	}
	return analyticsBucketWeek
}

type analyticsFloatDimension struct {
	Key   string
	Label string
	Value float64
}

func buildBarAnalytics(rows []store.RequestAnalyticsAggregate, timeZone config.TimeZone, bucketUnit string) analyticsBarBuildResult {
	modelCosts := map[string]float64{}
	modelLabels := map[string]string{}
	totalCost := 0.0
	missingCost := int64(0)
	for _, row := range rows {
		missingCost += row.MissingCostCount
		if !row.HasCost {
			continue
		}
		modelCosts[row.ModelKey] += row.Cost
		modelLabels[row.ModelKey] = row.ModelLabel
		totalCost += row.Cost
	}

	topModels := topFloatDimensionKeys(modelCosts, analyticsTopModels)
	modelMap := map[string]string{}
	for _, key := range topModels {
		modelMap[key] = key
	}
	for key := range modelCosts {
		if _, ok := modelMap[key]; !ok {
			modelMap[key] = "__other_model__"
		}
	}

	keyCosts := map[string]map[string]float64{}
	keyLabels := map[string]map[string]string{}
	for _, row := range rows {
		if !row.HasCost {
			continue
		}
		modelKey := modelMap[row.ModelKey]
		if keyCosts[modelKey] == nil {
			keyCosts[modelKey] = map[string]float64{}
			keyLabels[modelKey] = map[string]string{}
		}
		keyCosts[modelKey][row.APIKeyKey] += row.Cost
		keyLabels[modelKey][row.APIKeyKey] = row.APIKeyLabel
	}
	keyMap := map[string]map[string]string{}
	for modelKey, costs := range keyCosts {
		keyMap[modelKey] = map[string]string{}
		for _, key := range topFloatDimensionKeys(costs, analyticsTopKeys) {
			keyMap[modelKey][key] = key
		}
		for key := range costs {
			if _, ok := keyMap[modelKey][key]; !ok {
				keyMap[modelKey][key] = "__other_api_key__"
			}
		}
	}

	type segment struct {
		Key   string
		Label string
		Value float64
	}
	type group struct {
		Key      string
		Label    string
		Value    float64
		Segments map[string]*segment
	}
	type point struct {
		Start  time.Time
		Groups map[string]*group
	}
	pointMap := map[time.Time]*point{}
	for _, row := range rows {
		if !row.HasCost {
			continue
		}
		bucket := analyticsBucketStart(row.BucketStart, timeZone, bucketUnit)
		item := pointMap[bucket]
		if item == nil {
			item = &point{Start: bucket, Groups: map[string]*group{}}
			pointMap[bucket] = item
		}
		modelKey := modelMap[row.ModelKey]
		modelLabel := modelLabels[row.ModelKey]
		if modelKey == "__other_model__" {
			modelLabel = "其他"
		}
		itemGroup := item.Groups[modelKey]
		if itemGroup == nil {
			itemGroup = &group{Key: modelKey, Label: modelLabel, Segments: map[string]*segment{}}
			item.Groups[modelKey] = itemGroup
		}
		itemGroup.Value += row.Cost
		mappedKey := keyMap[modelKey][row.APIKeyKey]
		label := keyLabels[modelKey][row.APIKeyKey]
		if mappedKey == "__other_api_key__" {
			label = "其他"
		}
		itemSegment := itemGroup.Segments[mappedKey]
		if itemSegment == nil {
			itemSegment = &segment{Key: mappedKey, Label: label}
			itemGroup.Segments[mappedKey] = itemSegment
		}
		itemSegment.Value += row.Cost
	}

	orderedModels := append([]string{}, topModels...)
	if hasMappedOther(modelMap, "__other_model__") {
		orderedModels = append(orderedModels, "__other_model__")
	}
	series := make([]AnalyticsBarSeries, 0)
	for _, modelKey := range orderedModels {
		modelLabel := modelLabels[modelKey]
		if modelKey == "__other_model__" {
			modelLabel = "其他"
		}
		keys := make([]analyticsFloatDimension, 0, len(keyCosts[modelKey]))
		for key, value := range keyCosts[modelKey] {
			keys = append(keys, analyticsFloatDimension{Key: key, Label: keyLabels[modelKey][key], Value: value})
		}
		sort.SliceStable(keys, func(i, j int) bool {
			mappedI := keyMap[modelKey][keys[i].Key]
			mappedJ := keyMap[modelKey][keys[j].Key]
			if mappedI != mappedJ {
				return mappedI < mappedJ
			}
			return keys[i].Key < keys[j].Key
		})
		seenSeries := map[string]struct{}{}
		for _, key := range keys {
			mappedKey := keyMap[modelKey][key.Key]
			if _, ok := seenSeries[mappedKey]; ok {
				continue
			}
			seenSeries[mappedKey] = struct{}{}
			label := key.Label
			if mappedKey == "__other_api_key__" {
				label = "其他"
			}
			series = append(series, AnalyticsBarSeries{
				ModelKey: modelKey, ModelLabel: modelLabel,
				APIKeyID: mappedKey, APIKeyLabel: label,
				IsOther: modelKey == "__other_model__" || mappedKey == "__other_api_key__",
			})
		}
	}

	buckets := make([]time.Time, 0, len(pointMap))
	for bucket := range pointMap {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })
	points := make([]AnalyticsBarPoint, 0, len(buckets))
	for _, bucket := range buckets {
		item := pointMap[bucket]
		groups := make([]AnalyticsBarGroup, 0, len(item.Groups))
		for _, modelKey := range orderedModels {
			itemGroup := item.Groups[modelKey]
			if itemGroup == nil {
				continue
			}
			segments := make([]AnalyticsBarSegment, 0, len(itemGroup.Segments))
			for _, itemSegment := range itemGroup.Segments {
				segments = append(segments, AnalyticsBarSegment{
					APIKeyID: itemSegment.Key, APIKeyLabel: itemSegment.Label,
					Cost:    itemSegment.Value,
					IsOther: itemSegment.Key == "__other_api_key__" || modelKey == "__other_model__",
				})
			}
			sort.Slice(segments, func(i, j int) bool { return segments[i].APIKeyID < segments[j].APIKeyID })
			groups = append(groups, AnalyticsBarGroup{
				ModelKey: itemGroup.Key, ModelLabel: itemGroup.Label,
				TotalCost: itemGroup.Value, Segments: segments,
			})
		}
		points = append(points, AnalyticsBarPoint{
			BucketStart: bucket.Format(time.RFC3339Nano),
			BucketEnd:   analyticsBucketEnd(bucket, bucketUnit).Format(time.RFC3339Nano),
			Groups:      groups,
		})
	}
	return analyticsBarBuildResult{
		AnalyticsBarData: AnalyticsBarData{BucketUnit: bucketUnit, Series: series, Points: points},
		TotalCost:        totalCost, MissingCostRequests: missingCost,
	}
}

type analyticsSankeyDimensionTotal struct {
	Key   string
	Label string
	Value int64
}

func buildSankeyAnalytics(rows []store.RequestAnalyticsAggregate) AnalyticsSankeyData {
	totals := func(get func(store.RequestAnalyticsAggregate) (string, string)) map[string]*analyticsSankeyDimensionTotal {
		result := map[string]*analyticsSankeyDimensionTotal{}
		for _, row := range rows {
			key, label := get(row)
			key = strings.TrimSpace(key)
			label = strings.TrimSpace(label)
			if key == "" || strings.EqualFold(key, "none") {
				key = "__unknown__"
			}
			if label == "" || strings.EqualFold(label, "none") {
				label = "未识别"
			}
			item := result[key]
			if item == nil {
				item = &analyticsSankeyDimensionTotal{Key: key, Label: label}
				result[key] = item
			}
			item.Value += row.TotalTokens
		}
		return result
	}
	siteMap := sankeyDimensionMapping(totals(func(row store.RequestAnalyticsAggregate) (string, string) { return row.SiteKey, row.SiteLabel }))
	modelMap := sankeyDimensionMapping(totals(func(row store.RequestAnalyticsAggregate) (string, string) { return row.ModelKey, row.ModelLabel }))
	keyMap := sankeyDimensionMapping(totals(func(row store.RequestAnalyticsAggregate) (string, string) { return row.APIKeyKey, row.APIKeyLabel }))

	type edge struct{ Source, Target string }
	edges := map[edge]int64{}
	nodesByID := map[string]AnalyticsSankeyNode{}
	addNode := func(kind, key, label string, other bool) string {
		id := kind + ":" + key
		if _, ok := nodesByID[id]; !ok {
			nodesByID[id] = AnalyticsSankeyNode{ID: id, Label: label, Type: kind, IsOther: other}
		}
		return id
	}
	totalTokens := int64(0)
	for _, row := range rows {
		totalTokens += row.TotalTokens
		siteKey, siteLabel, siteOther := mappedSankeyDimension(row.SiteKey, row.SiteLabel, siteMap)
		modelKey, modelLabel, modelOther := mappedSankeyDimension(row.ModelKey, row.ModelLabel, modelMap)
		apiKey, apiLabel, apiOther := mappedSankeyDimension(row.APIKeyKey, row.APIKeyLabel, keyMap)
		siteID := addNode("site", siteKey, siteLabel, siteOther)
		modelID := addNode("model", modelKey, modelLabel, modelOther)
		apiID := addNode("api_key", apiKey, apiLabel, apiOther)
		edges[edge{Source: siteID, Target: modelID}] += row.TotalTokens
		edges[edge{Source: modelID, Target: apiID}] += row.TotalTokens
	}
	nodes := make([]AnalyticsSankeyNode, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return nodes[i].ID < nodes[j].ID
	})
	links := make([]AnalyticsSankeyLink, 0, len(edges))
	for item, value := range edges {
		links = append(links, AnalyticsSankeyLink{Source: item.Source, Target: item.Target, Value: value})
	}
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Source != links[j].Source {
			return links[i].Source < links[j].Source
		}
		return links[i].Target < links[j].Target
	})
	return AnalyticsSankeyData{Nodes: nodes, Links: links, TotalTokens: totalTokens}
}

func sankeyDimensionMapping(values map[string]*analyticsSankeyDimensionTotal) map[string]string {
	items := make([]analyticsSankeyDimensionTotal, 0, len(values))
	for _, value := range values {
		items = append(items, *value)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Value != items[j].Value {
			return items[i].Value > items[j].Value
		}
		return items[i].Key < items[j].Key
	})
	mapping := map[string]string{}
	for index, item := range items {
		if index < analyticsTopNodes {
			mapping[item.Key] = item.Key
		} else {
			mapping[item.Key] = "__other__"
		}
	}
	return mapping
}

func mappedSankeyDimension(key, label string, mapping map[string]string) (string, string, bool) {
	if strings.TrimSpace(key) == "" {
		key = "__unknown__"
	}
	if strings.TrimSpace(label) == "" {
		label = "未识别"
	}
	mapped, ok := mapping[key]
	if !ok {
		mapped = key
	}
	if mapped == "__other__" {
		return mapped, "其他", true
	}
	return mapped, label, false
}

func topFloatDimensionKeys(values map[string]float64, limit int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if values[keys[i]] != values[keys[j]] {
			return values[keys[i]] > values[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > limit {
		return keys[:limit]
	}
	return keys
}

func hasMappedOther(mapping map[string]string, other string) bool {
	for _, mapped := range mapping {
		if mapped == other {
			return true
		}
	}
	return false
}

func rebucketAnalyticsRowsByWeek(rows []store.RequestAnalyticsAggregate, timeZone config.TimeZone) []store.RequestAnalyticsAggregate {
	type rowKey struct {
		BucketStart time.Time
		SiteKey     string
		SiteLabel   string
		ModelKey    string
		ModelLabel  string
		APIKeyKey   string
		APIKeyLabel string
		Currency    string
		HasCost     bool
	}
	byKey := map[rowKey]*store.RequestAnalyticsAggregate{}
	for _, raw := range rows {
		row := raw
		row.BucketStart = timeZone.StartOfWeek(row.BucketStart)
		key := rowKey{row.BucketStart, row.SiteKey, row.SiteLabel, row.ModelKey, row.ModelLabel, row.APIKeyKey, row.APIKeyLabel, row.Currency, row.HasCost}
		item := byKey[key]
		if item == nil {
			copy := row
			byKey[key] = &copy
			continue
		}
		item.RequestCount += row.RequestCount
		item.TotalTokens += row.TotalTokens
		item.Cost += row.Cost
		item.MissingCostCount += row.MissingCostCount
		item.HasCost = item.HasCost || row.HasCost
	}
	result := make([]store.RequestAnalyticsAggregate, 0, len(byKey))
	for _, row := range byKey {
		result = append(result, *row)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].BucketStart.Before(result[j].BucketStart) })
	return result
}

func analyticsBucketStart(value time.Time, timeZone config.TimeZone, unit string) time.Time {
	switch unit {
	case analyticsBucketHour:
		return timeZone.StartOfHour(value)
	case analyticsBucketWeek:
		return timeZone.StartOfWeek(value)
	default:
		return timeZone.StartOfDay(value)
	}
}

func analyticsBucketEnd(value time.Time, unit string) time.Time {
	switch unit {
	case analyticsBucketHour:
		return value.Add(time.Hour)
	case analyticsBucketWeek:
		return value.AddDate(0, 0, 7)
	default:
		return value.AddDate(0, 0, 1)
	}
}

func uniqueAnalyticsUUIDs(values []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(values))
	seen := map[uuid.UUID]struct{}{}
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func timePtr(value time.Time) *time.Time { return &value }
