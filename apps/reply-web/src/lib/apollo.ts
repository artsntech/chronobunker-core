import { ApolloClient, InMemoryCache, HttpLink, from } from '@apollo/client';
import { SetContextLink } from '@apollo/client/link/context';

const httpLink = new HttpLink({
  uri: 'http://localhost:8080/query',
});

// v4: setter signature is (prevContext, operation)
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

export const client = new ApolloClient({
  link: from([authLink, httpLink]),
  cache: new InMemoryCache(),
});