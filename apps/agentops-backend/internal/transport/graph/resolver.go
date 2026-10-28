package graph

import "github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"

// Resolver is the root dependency container for GraphQL resolvers.
type Resolver struct {
	AgentRepo *agent.Repository
}

// queryResolver implements the generated QueryResolver interface.
type queryResolver struct{ *Resolver }

// mutationResolver implements the generated MutationResolver interface.
type mutationResolver struct{ *Resolver }

// agentResolver implements the generated AgentResolver interface.
// All Agent fields are resolved directly from the agent.Agent struct via
// autobind, so this resolver returns nil for every field.
type agentResolver struct{ *Resolver }

// Query returns the QueryResolver implementation.
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// Mutation returns the MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}

// Agent returns the AgentResolver implementation.
// Returning nil is valid because no custom field resolvers are defined for Agent.
// The autobind configuration maps every Agent field directly to the agent.Agent struct.
func (r *Resolver) Agent() AgentResolver {
	return nil
}