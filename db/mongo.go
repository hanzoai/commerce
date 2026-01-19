package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBConfig holds configuration for MongoDB/FerretDB
type MongoDBConfig struct {
	// URI is the connection string
	// MongoDB: mongodb://localhost:27017
	// FerretDB: mongodb://localhost:27017 (connects to FerretDB proxy)
	URI string

	// Database name
	Database string

	// TenantID for multi-tenant isolation
	TenantID string

	// TenantType is "user" or "org"
	TenantType string

	// MaxPoolSize for connection pooling
	MaxPoolSize uint64

	// MinPoolSize minimum connections
	MinPoolSize uint64

	// ConnectTimeout for initial connection
	ConnectTimeout time.Duration

	// EnableVectorSearch enables vector search (requires Atlas or compatible)
	EnableVectorSearch bool

	// VectorDimensions for embeddings
	VectorDimensions int
}

// MongoDB implements the DB interface using MongoDB or FerretDB
type MongoDB struct {
	config   *MongoDBConfig
	client   *mongo.Client
	database *mongo.Database
}

// NewMongoDB creates a new MongoDB/FerretDB connection
func NewMongoDB(cfg *MongoDBConfig) (*MongoDB, error) {
	if cfg == nil || cfg.URI == "" {
		return nil, errors.New("db: MongoDBConfig with URI is required")
	}

	if cfg.Database == "" {
		cfg.Database = "commerce"
	}

	// Build options
	opts := options.Client().ApplyURI(cfg.URI)

	if cfg.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize > 0 {
		opts.SetMinPoolSize(cfg.MinPoolSize)
	}
	if cfg.ConnectTimeout > 0 {
		opts.SetConnectTimeout(cfg.ConnectTimeout)
	}

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("db: failed to connect to mongodb: %w", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return nil, fmt.Errorf("db: failed to ping mongodb: %w", err)
	}

	mdb := &MongoDB{
		config:   cfg,
		client:   client,
		database: client.Database(cfg.Database),
	}

	// Create indexes
	if err := mdb.ensureIndexes(ctx); err != nil {
		client.Disconnect(ctx)
		return nil, fmt.Errorf("db: failed to create indexes: %w", err)
	}

	return mdb, nil
}

// ensureIndexes creates necessary indexes
func (db *MongoDB) ensureIndexes(ctx context.Context) error {
	// Index on tenant_id for multi-tenancy
	_, err := db.database.Collection("_entities").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "kind", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Index on kind for queries
	_, err = db.database.Collection("_entities").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "kind", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Index on parent_id for ancestor queries
	_, err = db.database.Collection("_entities").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "parent_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	return nil
}

// TenantID returns the tenant identifier
func (db *MongoDB) TenantID() string {
	return db.config.TenantID
}

// TenantType returns "user" or "org"
func (db *MongoDB) TenantType() string {
	return db.config.TenantType
}

// Close closes the database connection
func (db *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.client.Disconnect(ctx)
}

// mongoEntity is the document structure stored in MongoDB
type mongoEntity struct {
	ID        string                 `bson:"_id"`
	Kind      string                 `bson:"kind"`
	TenantID  string                 `bson:"tenant_id,omitempty"`
	ParentID  string                 `bson:"parent_id,omitempty"`
	Data      map[string]interface{} `bson:"data"`
	CreatedAt time.Time              `bson:"created_at"`
	UpdatedAt time.Time              `bson:"updated_at"`
	Deleted   bool                   `bson:"deleted"`
}

// Get retrieves an entity by key
func (db *MongoDB) Get(ctx context.Context, key Key, dst interface{}) error {
	if key == nil {
		return ErrInvalidKey
	}

	filter := bson.M{
		"_id":     key.Encode(),
		"kind":    key.Kind(),
		"deleted": false,
	}

	if db.config.TenantID != "" {
		filter["tenant_id"] = db.config.TenantID
	}

	var entity mongoEntity
	err := db.database.Collection("_entities").FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNoSuchEntity
		}
		return err
	}

	return mapToStruct(entity.Data, dst)
}

// Put stores an entity
func (db *MongoDB) Put(ctx context.Context, key Key, src interface{}) (Key, error) {
	if key == nil {
		return nil, ErrInvalidKey
	}

	data, err := structToMap(src)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	entity := mongoEntity{
		ID:        key.Encode(),
		Kind:      key.Kind(),
		TenantID:  db.config.TenantID,
		Data:      data,
		UpdatedAt: now,
		Deleted:   false,
	}

	if p := key.Parent(); p != nil {
		entity.ParentID = p.Encode()
	}

	opts := options.Replace().SetUpsert(true)
	filter := bson.M{"_id": key.Encode()}

	// Check if exists to preserve created_at
	var existing mongoEntity
	err = db.database.Collection("_entities").FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		entity.CreatedAt = existing.CreatedAt
	} else {
		entity.CreatedAt = now
	}

	_, err = db.database.Collection("_entities").ReplaceOne(ctx, filter, entity, opts)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// Delete removes an entity (soft delete)
func (db *MongoDB) Delete(ctx context.Context, key Key) error {
	if key == nil {
		return ErrInvalidKey
	}

	filter := bson.M{"_id": key.Encode(), "kind": key.Kind()}
	if db.config.TenantID != "" {
		filter["tenant_id"] = db.config.TenantID
	}

	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"updated_at": time.Now(),
		},
	}

	_, err := db.database.Collection("_entities").UpdateOne(ctx, filter, update)
	return err
}

// GetMulti retrieves multiple entities
func (db *MongoDB) GetMulti(ctx context.Context, keys []Key, dst interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.Encode()
	}

	filter := bson.M{
		"_id":     bson.M{"$in": ids},
		"deleted": false,
	}

	if db.config.TenantID != "" {
		filter["tenant_id"] = db.config.TenantID
	}

	cursor, err := db.database.Collection("_entities").Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Build result map
	results := make(map[string]map[string]interface{})
	for cursor.Next(ctx) {
		var entity mongoEntity
		if err := cursor.Decode(&entity); err != nil {
			return err
		}
		results[entity.ID] = entity.Data
	}

	// Unmarshal into destination slice
	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Ptr || dstVal.Elem().Kind() != reflect.Slice {
		return errors.New("db: dst must be a pointer to a slice")
	}

	sliceVal := dstVal.Elem()
	elemType := sliceVal.Type().Elem()

	for _, k := range keys {
		data, ok := results[k.Encode()]
		if !ok {
			sliceVal = reflect.Append(sliceVal, reflect.Zero(elemType))
			continue
		}

		elem := reflect.New(elemType.Elem())
		if err := mapToStruct(data, elem.Interface()); err != nil {
			return err
		}
		sliceVal = reflect.Append(sliceVal, elem)
	}

	dstVal.Elem().Set(sliceVal)
	return nil
}

// PutMulti stores multiple entities
func (db *MongoDB) PutMulti(ctx context.Context, keys []Key, src interface{}) ([]Key, error) {
	if len(keys) == 0 {
		return keys, nil
	}

	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Slice {
		return nil, errors.New("db: src must be a slice")
	}

	if srcVal.Len() != len(keys) {
		return nil, errors.New("db: keys and src must have same length")
	}

	models := make([]mongo.WriteModel, len(keys))
	now := time.Now()

	for i, key := range keys {
		data, err := structToMap(srcVal.Index(i).Interface())
		if err != nil {
			return nil, err
		}

		entity := mongoEntity{
			ID:        key.Encode(),
			Kind:      key.Kind(),
			TenantID:  db.config.TenantID,
			Data:      data,
			CreatedAt: now,
			UpdatedAt: now,
			Deleted:   false,
		}

		if p := key.Parent(); p != nil {
			entity.ParentID = p.Encode()
		}

		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": key.Encode()}).
			SetReplacement(entity).
			SetUpsert(true)
	}

	_, err := db.database.Collection("_entities").BulkWrite(ctx, models)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// DeleteMulti removes multiple entities
func (db *MongoDB) DeleteMulti(ctx context.Context, keys []Key) error {
	if len(keys) == 0 {
		return nil
	}

	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.Encode()
	}

	filter := bson.M{"_id": bson.M{"$in": ids}}
	if db.config.TenantID != "" {
		filter["tenant_id"] = db.config.TenantID
	}

	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"updated_at": time.Now(),
		},
	}

	_, err := db.database.Collection("_entities").UpdateMany(ctx, filter, update)
	return err
}

// Query returns a new query for the given kind
func (db *MongoDB) Query(kind string) Query {
	return &mongoQuery{
		db:       db,
		kind:     kind,
		tenantID: db.config.TenantID,
	}
}

// VectorSearch performs similarity search (requires vector-capable MongoDB)
func (db *MongoDB) VectorSearch(ctx context.Context, opts *VectorSearchOptions) ([]VectorResult, error) {
	if opts == nil || len(opts.Vector) == 0 {
		return nil, errors.New("db: VectorSearchOptions with Vector is required")
	}

	// This requires MongoDB Atlas Search or a vector-capable deployment
	// FerretDB doesn't support vector search natively
	return nil, errors.New("db: vector search not supported in this MongoDB deployment")
}

// PutVector stores a vector embedding
func (db *MongoDB) PutVector(ctx context.Context, kind string, id string, vector []float32, metadata map[string]interface{}) error {
	doc := bson.M{
		"_id":        id,
		"kind":       kind,
		"tenant_id":  db.config.TenantID,
		"embedding":  vector,
		"metadata":   metadata,
		"created_at": time.Now(),
	}

	opts := options.Replace().SetUpsert(true)
	_, err := db.database.Collection("_vectors").ReplaceOne(ctx, bson.M{"_id": id}, doc, opts)
	return err
}

// NewKey creates a new key
func (db *MongoDB) NewKey(kind string, stringID string, intID int64, parent Key) Key {
	return &mongoKey{
		kind:      kind,
		stringID:  stringID,
		intID:     intID,
		parent:    parent,
		namespace: db.config.TenantID,
	}
}

// NewIncompleteKey creates a key that will be assigned an ID on Put
func (db *MongoDB) NewIncompleteKey(kind string, parent Key) Key {
	return &mongoKey{
		kind:       kind,
		parent:     parent,
		namespace:  db.config.TenantID,
		incomplete: true,
	}
}

// AllocateIDs pre-allocates entity IDs
func (db *MongoDB) AllocateIDs(kind string, parent Key, n int) ([]Key, error) {
	keys := make([]Key, n)
	for i := 0; i < n; i++ {
		keys[i] = &mongoKey{
			kind:      kind,
			stringID:  primitive.NewObjectID().Hex(),
			parent:    parent,
			namespace: db.config.TenantID,
		}
	}
	return keys, nil
}

// RunInTransaction executes a function within a transaction
func (db *MongoDB) RunInTransaction(ctx context.Context, fn func(tx Transaction) error, opts *TransactionOptions) error {
	session, err := db.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		tx := &mongoTransaction{
			db:      db,
			session: sessCtx,
		}
		return nil, fn(tx)
	})

	return err
}

// mongoKey implements the Key interface
type mongoKey struct {
	kind       string
	stringID   string
	intID      int64
	parent     Key
	namespace  string
	incomplete bool
}

func (k *mongoKey) Kind() string      { return k.kind }
func (k *mongoKey) StringID() string  { return k.stringID }
func (k *mongoKey) IntID() int64      { return k.intID }
func (k *mongoKey) Parent() Key       { return k.parent }
func (k *mongoKey) Namespace() string { return k.namespace }
func (k *mongoKey) Incomplete() bool  { return k.incomplete }

func (k *mongoKey) Encode() string {
	if k.stringID != "" {
		return k.stringID
	}
	if k.intID != 0 {
		return fmt.Sprintf("%d", k.intID)
	}
	if k.incomplete {
		k.stringID = primitive.NewObjectID().Hex()
		k.incomplete = false
	}
	return k.stringID
}

func (k *mongoKey) Equal(other Key) bool {
	if other == nil {
		return false
	}
	return k.Kind() == other.Kind() && k.Encode() == other.Encode()
}

// mongoTransaction implements Transaction
type mongoTransaction struct {
	db      *MongoDB
	session mongo.SessionContext
}

func (t *mongoTransaction) Get(key Key, dst interface{}) error {
	filter := bson.M{
		"_id":     key.Encode(),
		"kind":    key.Kind(),
		"deleted": false,
	}

	if t.db.config.TenantID != "" {
		filter["tenant_id"] = t.db.config.TenantID
	}

	var entity mongoEntity
	err := t.db.database.Collection("_entities").FindOne(t.session, filter).Decode(&entity)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNoSuchEntity
		}
		return err
	}

	return mapToStruct(entity.Data, dst)
}

func (t *mongoTransaction) Put(key Key, src interface{}) (Key, error) {
	data, err := structToMap(src)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	entity := mongoEntity{
		ID:        key.Encode(),
		Kind:      key.Kind(),
		TenantID:  t.db.config.TenantID,
		Data:      data,
		CreatedAt: now,
		UpdatedAt: now,
		Deleted:   false,
	}

	if p := key.Parent(); p != nil {
		entity.ParentID = p.Encode()
	}

	opts := options.Replace().SetUpsert(true)
	_, err = t.db.database.Collection("_entities").ReplaceOne(t.session, bson.M{"_id": key.Encode()}, entity, opts)
	return key, err
}

func (t *mongoTransaction) Delete(key Key) error {
	filter := bson.M{"_id": key.Encode(), "kind": key.Kind()}
	if t.db.config.TenantID != "" {
		filter["tenant_id"] = t.db.config.TenantID
	}

	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"updated_at": time.Now(),
		},
	}

	_, err := t.db.database.Collection("_entities").UpdateOne(t.session, filter, update)
	return err
}

func (t *mongoTransaction) Query(kind string) Query {
	return &mongoQuery{
		db:       t.db,
		kind:     kind,
		tenantID: t.db.config.TenantID,
		session:  t.session,
	}
}

// mongoQuery implements Query for MongoDB
type mongoQuery struct {
	db       *MongoDB
	kind     string
	tenantID string
	session  mongo.SessionContext

	filters  []bson.M
	sorts    bson.D
	limit    int64
	skip     int64
	ancestor Key
}

func (q *mongoQuery) Filter(filterStr string, value interface{}) Query {
	field, op := parseFilterString(filterStr)
	return q.FilterField(field, op, value)
}

func (q *mongoQuery) FilterField(fieldPath string, op string, value interface{}) Query {
	newQ := q.clone()

	mongoOp := "$eq"
	switch op {
	case "=", "==":
		mongoOp = "$eq"
	case "!=", "<>":
		mongoOp = "$ne"
	case ">":
		mongoOp = "$gt"
	case ">=":
		mongoOp = "$gte"
	case "<":
		mongoOp = "$lt"
	case "<=":
		mongoOp = "$lte"
	}

	newQ.filters = append(newQ.filters, bson.M{
		"data." + fieldPath: bson.M{mongoOp: value},
	})
	return newQ
}

func (q *mongoQuery) Order(fieldPath string) Query {
	newQ := q.clone()
	if strings.HasPrefix(fieldPath, "-") {
		newQ.sorts = append(newQ.sorts, bson.E{Key: "data." + strings.TrimPrefix(fieldPath, "-"), Value: -1})
	} else {
		newQ.sorts = append(newQ.sorts, bson.E{Key: "data." + fieldPath, Value: 1})
	}
	return newQ
}

func (q *mongoQuery) OrderDesc(fieldPath string) Query {
	newQ := q.clone()
	newQ.sorts = append(newQ.sorts, bson.E{Key: "data." + fieldPath, Value: -1})
	return newQ
}

func (q *mongoQuery) Limit(limit int) Query {
	newQ := q.clone()
	newQ.limit = int64(limit)
	return newQ
}

func (q *mongoQuery) Offset(offset int) Query {
	newQ := q.clone()
	newQ.skip = int64(offset)
	return newQ
}

func (q *mongoQuery) Project(fieldNames ...string) Query {
	return q // Not implemented for simplicity
}

func (q *mongoQuery) Distinct() Query {
	return q // Not implemented for simplicity
}

func (q *mongoQuery) Ancestor(ancestor Key) Query {
	newQ := q.clone()
	newQ.ancestor = ancestor
	return newQ
}

func (q *mongoQuery) Start(cursor Cursor) Query {
	return q
}

func (q *mongoQuery) End(cursor Cursor) Query {
	return q
}

func (q *mongoQuery) buildFilter() bson.M {
	filter := bson.M{
		"kind":    q.kind,
		"deleted": false,
	}

	if q.tenantID != "" {
		filter["tenant_id"] = q.tenantID
	}

	if q.ancestor != nil {
		filter["parent_id"] = q.ancestor.Encode()
	}

	if len(q.filters) > 0 {
		filter["$and"] = q.filters
	}

	return filter
}

func (q *mongoQuery) GetAll(ctx context.Context, dst interface{}) ([]Key, error) {
	filter := q.buildFilter()
	opts := options.Find()

	if len(q.sorts) > 0 {
		opts.SetSort(q.sorts)
	}
	if q.limit > 0 {
		opts.SetLimit(q.limit)
	}
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}

	var cursor *mongo.Cursor
	var err error

	if q.session != nil {
		cursor, err = q.db.database.Collection("_entities").Find(q.session, filter, opts)
	} else {
		cursor, err = q.db.database.Collection("_entities").Find(ctx, filter, opts)
	}

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Ptr || dstVal.Elem().Kind() != reflect.Slice {
		return nil, errors.New("db: dst must be a pointer to a slice")
	}

	sliceVal := dstVal.Elem()
	elemType := sliceVal.Type().Elem()
	isPointer := elemType.Kind() == reflect.Ptr
	if isPointer {
		elemType = elemType.Elem()
	}

	var keys []Key
	for cursor.Next(ctx) {
		var entity mongoEntity
		if err := cursor.Decode(&entity); err != nil {
			return nil, err
		}

		elem := reflect.New(elemType)
		if err := mapToStruct(entity.Data, elem.Interface()); err != nil {
			return nil, err
		}

		if isPointer {
			sliceVal = reflect.Append(sliceVal, elem)
		} else {
			sliceVal = reflect.Append(sliceVal, elem.Elem())
		}

		keys = append(keys, &mongoKey{
			kind:      q.kind,
			stringID:  entity.ID,
			namespace: q.tenantID,
		})
	}

	dstVal.Elem().Set(sliceVal)
	return keys, cursor.Err()
}

func (q *mongoQuery) First(ctx context.Context, dst interface{}) (Key, error) {
	filter := q.buildFilter()
	opts := options.FindOne()

	if len(q.sorts) > 0 {
		opts.SetSort(q.sorts)
	}

	var entity mongoEntity
	var err error

	if q.session != nil {
		err = q.db.database.Collection("_entities").FindOne(q.session, filter, opts).Decode(&entity)
	} else {
		err = q.db.database.Collection("_entities").FindOne(ctx, filter, opts).Decode(&entity)
	}

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoSuchEntity
		}
		return nil, err
	}

	if err := mapToStruct(entity.Data, dst); err != nil {
		return nil, err
	}

	return &mongoKey{
		kind:      q.kind,
		stringID:  entity.ID,
		namespace: q.tenantID,
	}, nil
}

func (q *mongoQuery) Count(ctx context.Context) (int, error) {
	filter := q.buildFilter()

	var count int64
	var err error

	if q.session != nil {
		count, err = q.db.database.Collection("_entities").CountDocuments(q.session, filter)
	} else {
		count, err = q.db.database.Collection("_entities").CountDocuments(ctx, filter)
	}

	return int(count), err
}

func (q *mongoQuery) Keys(ctx context.Context) ([]Key, error) {
	filter := q.buildFilter()
	opts := options.Find().SetProjection(bson.M{"_id": 1})

	if len(q.sorts) > 0 {
		opts.SetSort(q.sorts)
	}
	if q.limit > 0 {
		opts.SetLimit(q.limit)
	}
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}

	var cursor *mongo.Cursor
	var err error

	if q.session != nil {
		cursor, err = q.db.database.Collection("_entities").Find(q.session, filter, opts)
	} else {
		cursor, err = q.db.database.Collection("_entities").Find(ctx, filter, opts)
	}

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var keys []Key
	for cursor.Next(ctx) {
		var result struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		keys = append(keys, &mongoKey{
			kind:      q.kind,
			stringID:  result.ID,
			namespace: q.tenantID,
		})
	}

	return keys, cursor.Err()
}

func (q *mongoQuery) Run(ctx context.Context) Iterator {
	filter := q.buildFilter()
	opts := options.Find()

	if len(q.sorts) > 0 {
		opts.SetSort(q.sorts)
	}
	if q.limit > 0 {
		opts.SetLimit(q.limit)
	}
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}

	var cursor *mongo.Cursor
	var err error

	if q.session != nil {
		cursor, err = q.db.database.Collection("_entities").Find(q.session, filter, opts)
	} else {
		cursor, err = q.db.database.Collection("_entities").Find(ctx, filter, opts)
	}

	return &mongoIterator{
		cursor:    cursor,
		err:       err,
		kind:      q.kind,
		namespace: q.tenantID,
		ctx:       ctx,
	}
}

func (q *mongoQuery) clone() *mongoQuery {
	newQ := *q
	newQ.filters = append([]bson.M{}, q.filters...)
	newQ.sorts = append(bson.D{}, q.sorts...)
	return &newQ
}

// mongoIterator implements Iterator
type mongoIterator struct {
	cursor    *mongo.Cursor
	err       error
	kind      string
	namespace string
	ctx       context.Context
	offset    int
}

func (it *mongoIterator) Next(dst interface{}) (Key, error) {
	if it.err != nil {
		return nil, it.err
	}

	if it.cursor == nil || !it.cursor.Next(it.ctx) {
		if it.cursor != nil {
			if err := it.cursor.Err(); err != nil {
				return nil, err
			}
		}
		return nil, errors.New("db: no more results")
	}

	var entity mongoEntity
	if err := it.cursor.Decode(&entity); err != nil {
		return nil, err
	}

	if err := mapToStruct(entity.Data, dst); err != nil {
		return nil, err
	}

	it.offset++

	return &mongoKey{
		kind:      it.kind,
		stringID:  entity.ID,
		namespace: it.namespace,
	}, nil
}

func (it *mongoIterator) Cursor() (Cursor, error) {
	return &sqliteCursor{
		id:     fmt.Sprintf("%d", it.offset),
		offset: it.offset,
	}, nil
}

// Helper functions

func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = bson.Unmarshal(data, &result)
	return result, err
}

func mapToStruct(m map[string]interface{}, v interface{}) error {
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}
	return bson.Unmarshal(data, v)
}
