package graph

import (
	"database/sql"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

// Resolver is the root dependency container for GraphQL resolvers.
type Resolver struct {
	AgentRepo    *agent.Repository
	Router       *router.Router
	DB           *sql.DB
	TokenService *security.TokenService
	Gateway      *security.Gateway
}

type queryResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }

func (r *Resolver) Query() QueryResolver       { return &queryResolver{r} }
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }