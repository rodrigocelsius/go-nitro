package gen

import "testing"

func TestParseRoutes_ExtractsAnnotatedHandlers(t *testing.T) {
	routes, err := ParseRoutes("routetestdata/handlers.go")
	if err != nil {
		t.Fatalf("ParseRoutes failed: %v", err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 annotated routes (NotAnnotated should be excluded), got %d: %+v", len(routes), routes)
	}

	byName := make(map[string]RouteAnnotation)
	for _, r := range routes {
		byName[r.FuncName] = r
	}

	getUser, ok := byName["GetUser"]
	if !ok {
		t.Fatal("GetUser route not found")
	}
	if getUser.Method != "GET" || getUser.Path != "/users/:id" {
		t.Fatalf("unexpected GetUser route: %+v", getUser)
	}
	if getUser.CtxParam != "c" {
		t.Fatalf("expected ctx param name 'c', got %q", getUser.CtxParam)
	}
	if len(getUser.PathParams) != 1 || getUser.PathParams[0].Name != "id" || getUser.PathParams[0].Kind != KindInt {
		t.Fatalf("unexpected GetUser params: %+v", getUser.PathParams)
	}

	getOrderItem, ok := byName["GetOrderItem"]
	if !ok {
		t.Fatal("GetOrderItem route not found")
	}
	if len(getOrderItem.PathParams) != 2 {
		t.Fatalf("expected 2 params, got %+v", getOrderItem.PathParams)
	}
	if getOrderItem.PathParams[0].Name != "orderId" || getOrderItem.PathParams[1].Name != "itemId" {
		t.Fatalf("unexpected param order: %+v", getOrderItem.PathParams)
	}

	getProduct, ok := byName["GetProduct"]
	if !ok {
		t.Fatal("GetProduct route not found")
	}
	if getProduct.PathParams[0].Kind != KindString {
		t.Fatalf("expected string kind for slug, got %v", getProduct.PathParams[0].Kind)
	}
}

func TestParseRoutes_IgnoresNonAnnotatedFunctions(t *testing.T) {
	routes, err := ParseRoutes("routetestdata/handlers.go")
	if err != nil {
		t.Fatalf("ParseRoutes failed: %v", err)
	}
	for _, r := range routes {
		if r.FuncName == "NotAnnotated" {
			t.Fatal("NotAnnotated should not have been extracted")
		}
	}
}

func TestParseRoutes_ErrorsOnParamNameMismatch(t *testing.T) {
	_, err := ParseRoutes("routetestdata_invalid/mismatch.go")
	if err == nil {
		t.Fatal("expected error for mismatched param name (userId vs :id), got nil")
	}
}
