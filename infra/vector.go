// Package infra provides infrastructure clients.
//
// This file implements the Qdrant vector database client for product
// embeddings and semantic search.
package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzoai/vector-go/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// VectorConfig holds Qdrant configuration
type VectorConfig struct {
	// Enabled enables the vector service
	Enabled bool

	// Host is the Qdrant server host
	Host string

	// Port is the Qdrant gRPC port (default: 6334)
	Port int

	// APIKey for authentication (optional)
	APIKey string

	// UseTLS enables TLS connection
	UseTLS bool

	// DefaultCollection is the default collection name
	DefaultCollection string

	// DefaultDimensions is the default vector dimensions (1536 for OpenAI)
	DefaultDimensions uint64
}

// VectorClient wraps the Qdrant client
type VectorClient struct {
	config *VectorConfig
	conn   *grpc.ClientConn
	points qdrant.PointsClient
	colls  qdrant.CollectionsClient
}

// NewVectorClient creates a new Qdrant vector client
func NewVectorClient(ctx context.Context, cfg *VectorConfig) (*VectorClient, error) {
	if cfg.Port == 0 {
		cfg.Port = 6334
	}
	if cfg.DefaultDimensions == 0 {
		cfg.DefaultDimensions = 1536
	}
	if cfg.DefaultCollection == "" {
		cfg.DefaultCollection = "products"
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Setup gRPC options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if cfg.APIKey != "" {
		// Add API key authentication if provided
		opts = append(opts, grpc.WithPerRPCCredentials(&apiKeyAuth{apiKey: cfg.APIKey}))
	}

	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant: %w", err)
	}

	client := &VectorClient{
		config: cfg,
		conn:   conn,
		points: qdrant.NewPointsClient(conn),
		colls:  qdrant.NewCollectionsClient(conn),
	}

	return client, nil
}

// apiKeyAuth implements gRPC PerRPCCredentials for API key auth
type apiKeyAuth struct {
	apiKey string
}

func (a *apiKeyAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"api-key": a.apiKey}, nil
}

func (a *apiKeyAuth) RequireTransportSecurity() bool {
	return false
}

// EnsureCollection creates a collection if it doesn't exist
func (c *VectorClient) EnsureCollection(ctx context.Context, name string, dimensions uint64) error {
	if dimensions == 0 {
		dimensions = c.config.DefaultDimensions
	}

	// Check if collection exists
	_, err := c.colls.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: name,
	})
	if err == nil {
		return nil // Collection exists
	}

	// Create collection
	_, err = c.colls.Create(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     dimensions,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// Upsert inserts or updates vectors
func (c *VectorClient) Upsert(ctx context.Context, collection string, points []*VectorPoint) error {
	if collection == "" {
		collection = c.config.DefaultCollection
	}

	qdrantPoints := make([]*qdrant.PointStruct, len(points))
	for i, p := range points {
		qdrantPoints[i] = &qdrant.PointStruct{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Uuid{Uuid: p.ID},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{
					Vector: &qdrant.Vector{Data: p.Vector},
				},
			},
			Payload: toQdrantPayload(p.Payload),
		}
	}

	_, err := c.points.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         qdrantPoints,
		Wait:           boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	return nil
}

// Search performs vector similarity search
func (c *VectorClient) Search(ctx context.Context, opts *VectorSearchOpts) ([]VectorSearchResult, error) {
	if opts.Collection == "" {
		opts.Collection = c.config.DefaultCollection
	}
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	req := &qdrant.SearchPoints{
		CollectionName: opts.Collection,
		Vector:         opts.Vector,
		Limit:          uint64(opts.Limit),
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
		ScoreThreshold: &opts.MinScore,
	}

	// Add filters if provided
	if opts.Filter != nil {
		req.Filter = buildQdrantFilter(opts.Filter)
	}

	resp, err := c.points.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	results := make([]VectorSearchResult, len(resp.Result))
	for i, r := range resp.Result {
		results[i] = VectorSearchResult{
			ID:      getPointID(r.Id),
			Score:   r.Score,
			Payload: fromQdrantPayload(r.Payload),
		}
	}

	return results, nil
}

// Delete removes vectors by ID
func (c *VectorClient) Delete(ctx context.Context, collection string, ids []string) error {
	if collection == "" {
		collection = c.config.DefaultCollection
	}

	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Uuid{Uuid: id},
		}
	}

	_, err := c.points.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{Ids: pointIDs},
			},
		},
		Wait: boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	return nil
}

// Health checks the Qdrant connection
func (c *VectorClient) Health(ctx context.Context) HealthStatus {
	start := time.Now()

	_, err := c.colls.List(ctx, &qdrant.ListCollectionsRequest{})
	if err != nil {
		return HealthStatus{
			Healthy: false,
			Latency: time.Since(start),
			Error:   err.Error(),
		}
	}

	return HealthStatus{
		Healthy: true,
		Latency: time.Since(start),
	}
}

// Close closes the Qdrant connection
func (c *VectorClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// VectorPoint represents a point to insert
type VectorPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

// VectorSearchOpts configures vector search
type VectorSearchOpts struct {
	Collection string
	Vector     []float32
	Limit      int
	MinScore   float32
	Filter     map[string]interface{}
}

// VectorSearchResult represents a search result
type VectorSearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func toQdrantPayload(payload map[string]interface{}) map[string]*qdrant.Value {
	if payload == nil {
		return nil
	}

	result := make(map[string]*qdrant.Value)
	for k, v := range payload {
		result[k] = toQdrantValue(v)
	}
	return result
}

func toQdrantValue(v interface{}) *qdrant.Value {
	switch val := v.(type) {
	case string:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: val}}
	case int:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}
	case int64:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: val}}
	case float64:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: val}}
	case float32:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: float64(val)}}
	case bool:
		return &qdrant.Value{Kind: &qdrant.Value_BoolValue{BoolValue: val}}
	default:
		return &qdrant.Value{Kind: &qdrant.Value_NullValue{}}
	}
}

func fromQdrantPayload(payload map[string]*qdrant.Value) map[string]interface{} {
	if payload == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range payload {
		result[k] = fromQdrantValue(v)
	}
	return result
}

func fromQdrantValue(v *qdrant.Value) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	default:
		return nil
	}
}

func getPointID(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	switch val := id.PointIdOptions.(type) {
	case *qdrant.PointId_Uuid:
		return val.Uuid
	case *qdrant.PointId_Num:
		return fmt.Sprintf("%d", val.Num)
	default:
		return ""
	}
}

func buildQdrantFilter(filter map[string]interface{}) *qdrant.Filter {
	if filter == nil {
		return nil
	}

	conditions := make([]*qdrant.Condition, 0)
	for key, value := range filter {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: key,
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{
							Keyword: fmt.Sprintf("%v", value),
						},
					},
				},
			},
		})
	}

	return &qdrant.Filter{
		Must: conditions,
	}
}
