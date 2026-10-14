package field

import "github.com/rodrigocelsius/go-nitro/blinq/v2/expr"

// Relation describes HasMany / HasOne for Include (anti-N+1 batch).
type Relation struct {
	Name       string
	Target     Entity
	LocalKey   expr.FieldRef
	ForeignKey expr.FieldRef
}

// HasMany creates a 1:N relation (parent.LocalKey → child.ForeignKey).
func HasMany(name string, target Entity, localKey, foreignKey expr.FieldRef) Relation {
	return Relation{
		Name:       name,
		Target:     target,
		LocalKey:   localKey,
		ForeignKey: foreignKey,
	}
}

// HasOne is a semantic alias of HasMany (same batch; hydrate with a 0–1 slice).
func HasOne(name string, target Entity, localKey, foreignKey expr.FieldRef) Relation {
	return HasMany(name, target, localKey, foreignKey)
}

// BelongsTo describes N:1 from the child side (child.LocalKey FK → parent.ForeignKey PK).
// Same Include batching as HasOne.
//
//	orders.Include(field.BelongsTo("user", Users, Orders.UserID.Ref(), Users.ID.Ref()))
func BelongsTo(name string, target Entity, localKey, foreignKey expr.FieldRef) Relation {
	return HasOne(name, target, localKey, foreignKey)
}

// ToInclude converts to an AST node.
func (r Relation) ToInclude() expr.Include {
	return expr.Include{
		Name:       r.Name,
		Target:     r.Target.Ref(),
		LocalKey:   r.LocalKey,
		ForeignKey: r.ForeignKey,
	}
}
