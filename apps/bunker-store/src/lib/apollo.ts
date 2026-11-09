import { ApolloClient, InMemoryCache, HttpLink } from '@apollo/client'
import { SetContextLink } from '@apollo/client/link/context'

const httpLink = new HttpLink({ uri: '/query' })

const authLink = new SetContextLink((prevContext, _operation) => {
  const token = localStorage.getItem('bunker_jwt')
  return {
    ...prevContext,
    headers: {
      ...prevContext.headers,
      authorization: token ? `Bearer ${token}` : '',
    },
  }
})

export const client = new ApolloClient({
  link: authLink.concat(httpLink),
  cache: new InMemoryCache(),
})