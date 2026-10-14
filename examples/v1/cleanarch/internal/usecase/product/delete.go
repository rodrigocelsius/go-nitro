package product

import "context"

func Delete(ctx context.Context, repo Repository, id string) error {
	return repo.Delete(ctx, id)
}
