package expr

// WalkPred visits predicates depth-first (pre-order). fn may return false to skip children.
func WalkPred(p Predicate, fn func(Predicate) bool) {
	if p == nil || fn == nil {
		return
	}
	if !fn(p) {
		return
	}
	switch n := p.(type) {
	case And:
		for _, it := range n.Items {
			WalkPred(it, fn)
		}
	case Or:
		for _, it := range n.Items {
			WalkPred(it, fn)
		}
	case Not:
		WalkPred(n.Inner, fn)
	case Exists:
		WalkQuery(n.Sub, fn)
	case Binary:
		if sub, ok := asQueryValue(n.Value); ok {
			WalkQuery(sub, fn)
		}
	}
}

// WalkQuery visits Where/Having and nested predicates (Exists / IN subquery).
func WalkQuery(q Query, fn func(Predicate) bool) {
	if fn == nil {
		return
	}
	WalkPred(q.Where, fn)
	WalkPred(q.Having, fn)
	for _, j := range q.Joins {
		_ = j // joins are not Predicate; keys live on FieldRefs
	}
}

func asQueryValue(v any) (Query, bool) {
	switch t := v.(type) {
	case Subquery:
		return t.Query, true
	case *Query:
		if t != nil {
			return *t, true
		}
	case Query:
		return t, true
	}
	return Query{}, false
}
