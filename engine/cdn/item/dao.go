package item

import (
	"context"
	"strconv"
	"time"

	"github.com/go-gorp/gorp"
	"github.com/lib/pq"

	"github.com/ovh/cds/engine/gorpmapper"
	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/log"
)

func getItems(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, q gorpmapper.Query, opts ...gorpmapper.GetOptionFunc) ([]Item, error) {
	var res []Item
	if err := m.GetAll(ctx, db, q, &res, opts...); err != nil {
		return nil, err
	}

	var verifiedItems []Item
	for _, i := range res {
		isValid, err := m.CheckSignature(i, i.Signature)
		if err != nil {
			return nil, err
		}
		if !isValid {
			log.Error(ctx, "item.get> item %s data corrupted", i.ID)
			continue
		}
		verifiedItems = append(verifiedItems, i)
	}

	return verifiedItems, nil
}

func getItem(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, q gorpmapper.Query, opts ...gorpmapper.GetOptionFunc) (*Item, error) {
	var i Item
	found, err := m.Get(ctx, db, q, &i, opts...)
	if err != nil {
		return nil, sdk.WrapError(err, "cannot get item")
	}
	if !found {
		return nil, sdk.WithStack(sdk.ErrNotFound)
	}

	isValid, err := m.CheckSignature(i, i.Signature)
	if err != nil {
		return nil, err
	}
	if !isValid {
		log.Error(ctx, "item.get> item %s data corrupted", i.ID)
		return nil, sdk.WithStack(sdk.ErrNotFound)
	}
	return &i, nil
}

func LoadIDsToDelete(db gorp.SqlExecutor, size int) ([]string, error) {
	query := `
		SELECT id
		FROM item
		WHERE to_delete = true
		ORDER BY last_modified ASC
		LIMIT $1
	`
	var ids []string
	if _, err := db.Select(&ids, query, size); err != nil {
		return nil, sdk.WithStack(err)
	}
	return ids, nil
}

func DeleteByIDs(db gorp.SqlExecutor, ids []string) error {
	query := `
		DELETE FROM item WHERE id = ANY($1)
	`
	_, err := db.Exec(query, pq.StringArray(ids))
	return sdk.WithStack(err)
}

func LoadAll(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, size int, opts ...gorpmapper.GetOptionFunc) ([]Item, error) {
	query := gorpmapper.NewQuery("SELECT * FROM item ORDER BY created LIMIT $1").Args(size)
	return getItems(ctx, m, db, query, opts...)
}

// LoadByID returns an item from database for given id.
func LoadByID(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, id string, opts ...gorpmapper.GetOptionFunc) (*Item, error) {
	query := gorpmapper.NewQuery("SELECT * FROM item WHERE id = $1").Args(id)
	return getItem(ctx, m, db, query, opts...)
}

// LoadByIDs returns items from database for given ids.
func LoadByIDs(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, ids []string, opts ...gorpmapper.GetOptionFunc) ([]Item, error) {
	query := gorpmapper.NewQuery("SELECT * FROM item WHERE id = ANY($1)").Args(pq.StringArray(ids))
	return getItems(ctx, m, db, query, opts...)
}

// LoadAndLockByID returns an item from database for given id.
func LoadAndLockByID(ctx context.Context, m *gorpmapper.Mapper, db gorpmapper.SqlExecutorWithTx, id string, opts ...gorpmapper.GetOptionFunc) (*Item, error) {
	query := gorpmapper.NewQuery("SELECT * FROM item WHERE id = $1 FOR UPDATE SKIP LOCKED").Args(id)
	return getItem(ctx, m, db, query, opts...)
}

// Insert in database.
func Insert(ctx context.Context, m *gorpmapper.Mapper, db gorpmapper.SqlExecutorWithTx, i *Item) error {
	i.ID = sdk.UUID()
	i.Created = time.Now()
	i.LastModified = time.Now()
	if err := m.InsertAndSign(ctx, db, i); err != nil {
		return sdk.WrapError(err, "unable to insert item item %s", i.ID)
	}
	return nil
}

// Update in database
func Update(ctx context.Context, m *gorpmapper.Mapper, db gorpmapper.SqlExecutorWithTx, i *Item) error {
	i.LastModified = time.Now()
	if err := m.UpdateAndSign(ctx, db, i); err != nil {
		return sdk.WrapError(err, "unable to update item item")
	}
	return nil
}

func MarkToDeleteByWorkflowID(db gorpmapper.SqlExecutorWithTx, workflowID int64) error {
	query := `
		UPDATE item SET to_delete = true WHERE (api_ref->>'workflow_id')::int = $1 
	`
	_, err := db.Exec(query, workflowID)
	return sdk.WrapError(err, "unable to mark item to delete for workflow %s", strconv.Itoa(int(workflowID)))
}

func MarkToDeleteByRunIDs(db gorpmapper.SqlExecutorWithTx, runID int64) error {
	query := `
		UPDATE item SET to_delete = true WHERE (api_ref->>'run_id')::int = $1 
	`
	_, err := db.Exec(query, runID)
	return sdk.WrapError(err, "unable to mark item to delete for run %d", runID)
}

// LoadByAPIRefHashAndType load an item by his job id, step order and type
func LoadByAPIRefHashAndType(ctx context.Context, m *gorpmapper.Mapper, db gorp.SqlExecutor, hash string, itemType sdk.CDNItemType, opts ...gorpmapper.GetOptionFunc) (*Item, error) {
	query := gorpmapper.NewQuery(`
		SELECT *
		FROM item
		WHERE
			api_ref_hash = $1 AND
			type = $2
	`).Args(hash, itemType)
	return getItem(ctx, m, db, query)
}

func ComputeSizeByIDs(db gorp.SqlExecutor, itemIDs []string) (int64, error) {
	query := `
		SELECT SUM(size) FROM item
		WHERE id = ANY($1) 
	`
	size, err := db.SelectInt(query, pq.StringArray(itemIDs))
	if err != nil {
		return 0, sdk.WithStack(err)
	}
	return size, nil
}