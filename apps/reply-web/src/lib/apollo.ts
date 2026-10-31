import { ApolloClient, InMemoryCache, HttpLink, split } from '@apollo/client';
import { SetContextLink } from '@apollo/client/link/context';
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { getMainDefinition } from '@apollo/client/utilities';
import { createClient } from 'graphql-ws';

const httpLink = new HttpLink({ uri: 'http://localhost:8080/query' });

const authLink = new SetContextLink((prevContext, _operation) => {
  const token = localStorage.getItem('chrono_jwt');
  return {
    ...prevContext,
    headers: {
      ...prevContext.headers,
      authorization: token ? `Bearer ${token}` : '',
    },
  };
});

const wsLink = new GraphQLWsLink(
  createClient({
    url: 'ws://localhost:8080/query',
    connectionParams: () => ({
      Authorization: `Bearer ${localStorage.getItem('chrono_jwt') || ''}`,
    }),
  })
);

const splitLink = split(
  ({ query }) => {
    const def = getMainDefinition(query);
    return def.kind === 'OperationDefinition' && def.operation === 'subscription';
  },
  wsLink,
  authLink.concat(httpLink)
);

export const client = new ApolloClient({
  link: splitLink,
  cache: new InMemoryCache(),
});