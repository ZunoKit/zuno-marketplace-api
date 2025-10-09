package middleware

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DepthLimiter is a middleware that limits the depth of GraphQL queries
type DepthLimiter struct {
	MaxDepth int
}

// ExtensionName returns the name of the extension
func (d *DepthLimiter) ExtensionName() string {
	return "DepthLimiter"
}

// Validate is called when the query is parsed
func (d *DepthLimiter) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation intercepts the operation before execution
func (d *DepthLimiter) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	oc := graphql.GetOperationContext(ctx)

	// Calculate query depth
	depth := d.calculateDepth(oc.Operation.SelectionSet, 0)

	// Check if depth exceeds limit
	if depth > d.MaxDepth {
		return func(ctx context.Context) *graphql.Response {
			return &graphql.Response{
				Errors: gqlerror.List{
					&gqlerror.Error{
						Message: fmt.Sprintf("query depth %d exceeds maximum allowed depth of %d", depth, d.MaxDepth),
					},
				},
			}
		}
	}

	return next(ctx)
}

// calculateDepth recursively calculates the depth of a selection set
func (d *DepthLimiter) calculateDepth(selectionSet ast.SelectionSet, currentDepth int) int {
	if len(selectionSet) == 0 {
		return currentDepth
	}

	maxDepth := currentDepth
	for _, selection := range selectionSet {
		var depth int

		switch sel := selection.(type) {
		case *ast.Field:
			// Regular field selection
			depth = d.calculateDepth(sel.SelectionSet, currentDepth+1)
		case *ast.InlineFragment:
			// Inline fragment
			depth = d.calculateDepth(sel.SelectionSet, currentDepth)
		case *ast.FragmentSpread:
			// Fragment spread - we'll count this as current depth
			// In a production system, you'd want to resolve the fragment
			depth = currentDepth
		}

		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// QueryComplexityCalculator calculates the complexity of a query field
type QueryComplexityCalculator struct {
	// FieldCosts defines the cost for each field
	FieldCosts map[string]int
	// DefaultCost is the default cost for fields not in FieldCosts
	DefaultCost int
	// ListMultiplier is multiplied by the list size argument
	ListMultiplier int
}

// NewQueryComplexityCalculator creates a new complexity calculator with default costs
func NewQueryComplexityCalculator() *QueryComplexityCalculator {
	return &QueryComplexityCalculator{
		FieldCosts: map[string]int{
			// Auth operations
			"signInSiwe":     1,
			"verifySiwe":     5,
			"refreshSession": 2,
			"logout":         1,

			// User operations
			"me":         1,
			"user":       2,
			"users":      10,
			"createUser": 5,
			"updateUser": 3,

			// Wallet operations
			"myWallets":        5,
			"wallet":           2,
			"wallets":          10,
			"linkWallet":       5,
			"unlinkWallet":     3,
			"setPrimaryWallet": 2,

			// Collection operations
			"collection":         5,
			"collections":        20,
			"myCollections":      15,
			"prepareCollection":  10,
			"submitCollectionTx": 5,

			// NFT operations
			"nft":          3,
			"nfts":         15,
			"myNfts":       20,
			"prepareMint":  10,
			"submitMintTx": 5,

			// Chain operations
			"chain":       1,
			"chains":      5,
			"chainConfig": 2,

			// Media operations
			"uploadMedia":    10,
			"uploadToIPFS":   15,
			"getMediaStatus": 2,

			// Marketplace operations
			"listing":       3,
			"listings":      20,
			"offer":         3,
			"offers":        15,
			"createListing": 10,
			"cancelListing": 5,
			"createOffer":   10,
			"acceptOffer":   10,
			"cancelOffer":   5,
		},
		DefaultCost:    1,
		ListMultiplier: 2,
	}
}

// Calculate calculates the complexity of a field
func (c *QueryComplexityCalculator) Calculate(field *ast.Field, childComplexity int) int {
	// Get base cost for the field
	cost, ok := c.FieldCosts[field.Name]
	if !ok {
		cost = c.DefaultCost
	}

	// Add child complexity
	cost += childComplexity

	// Check for list arguments and multiply
	for _, arg := range field.Arguments {
		switch arg.Name {
		case "first", "last", "limit":
			if arg.Value != nil && arg.Value.Kind == ast.IntValue {
				if intVal, err := arg.Value.Value(nil); err == nil {
					if limit, ok := intVal.(int64); ok && limit > 0 {
						cost = cost * int(limit) * c.ListMultiplier / 10
					}
				}
			}
		}
	}

	return cost
}
