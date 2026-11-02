import { useState, useMemo } from 'react'
import { gql } from '@apollo/client'
import { useQuery, useMutation } from '@apollo/client/react'
import { ShoppingCart, LogOut, Package, MessageCircle, X } from 'lucide-react'

const GET_ORDERS = gql`
  query GetOrders {
    orders {
      id
      product
      quantity
      totalUsd
      status
      createdAt
    }
  }
`

const CREATE_ORDER = gql`
  mutation CreateOrder($input: CreateOrderInput!) {
    createOrder(input: $input) {
      id
      product
      totalUsd
      status
    }
  }
`

const INVOKE_AGENT = gql`
  mutation InvokeAgent($input: InvokeAgentInput!) {
    invokeAgent(input: $input) {
      completion
      modelId
      latencyMs
    }
  }
`

const PRODUCTS = [
  { name: 'Bunker Starter Plan', price: 99, description: 'Essential multi-tenant isolation for small teams.' },
  { name: 'Bunker Pro Plan', price: 299, description: 'Full AgentOps platform with RLS, Ed25519, and audit trail.' },
  { name: 'Bunker Enterprise Plan', price: 1299, description: 'Multi-region deployment with SSO and SIEM export.' },
  { name: 'Chrono Agent License', price: 149, description: 'One pre-configured AI agent (support, refunds, or proofs).' },
]

interface Order {
  id: string
  product: string
  quantity: number
  totalUsd: number
  status: string
  createdAt: string
}

interface OrdersData {
  orders: Order[]
}

interface CreateOrderData {
  createOrder: { id: string; product: string; totalUsd: number; status: string }
}

interface InvokeAgentData {
  invokeAgent: { completion: string; modelId: string; latencyMs: number }
}

export const Store = ({ onLogout }: { onLogout: () => void }) => {
  const [cart, setCart] = useState<Record<string, number>>({})
  const [chatOpen, setChatOpen] = useState(false)
  const [chatInput, setChatInput] = useState('')
  const [chatLog, setChatLog] = useState<string[]>([])
  const [sessionId] = useState(() => `store-${Date.now()}`)
  const [agentId, setAgentId] = useState<string | null>(null)

  const { data, loading, refetch } = useQuery(GET_ORDERS, { fetchPolicy: 'network-only' })
  const [createOrder, { loading: creating }] = useMutation(CREATE_ORDER)
  const [invokeAgent, { loading: chatting }] = useMutation(INVOKE_AGENT)

  const { data: agentsData } = useQuery(gql`
    query { agents { id name department } }
  `)

  useMemo(() => {
    const agents = (agentsData as any)?.agents as Array<{ id: string; name: string; department: string }> | undefined
    if (agents && !agentId) {
      const supportAgent = agents.find((a) => a.department === 'support')
      if (supportAgent) setAgentId(supportAgent.id)
    }
  }, [agentsData, agentId])

  const orders = (data as OrdersData | undefined)?.orders || []
  const cartItems = Object.entries(cart)
    .map(([name, quantity]) => {
      const product = PRODUCTS.find((p) => p.name === name)
      return product ? { ...product, quantity } : null
    })
    .filter(Boolean) as Array<{ name: string; price: number; quantity: number }>

  const cartTotal = cartItems.reduce((sum, item) => sum + item.price * item.quantity, 0)

  const addToCart = (name: string) => {
    setCart((prev) => ({ ...prev, [name]: (prev[name] || 0) + 1 }))
  }

  const placeOrder = async () => {
  if (cartItems.length === 0) return
  try {
    for (const item of cartItems) {
      const result = await createOrder({
        variables: { input: { product: item.name, quantity: item.quantity } },
      })
      const typed = result.data as CreateOrderData | undefined
      if (!typed?.createOrder) {
        throw new Error('Order creation returned no data')
      }
    }
    setCart({})
    await refetch()
  } catch (err: any) {
    console.error('Checkout failed:', err)
    alert(`Checkout failed: ${err.message}`)
  }
}

  const sendChat = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!chatInput.trim() || !agentId) return
    const userMsg = chatInput
    setChatLog((prev) => [...prev, `You: ${userMsg}`])
    setChatInput('')
    try {
      const result = await invokeAgent({
        variables: {
          input: { agentId, prompt: userMsg, sessionId },
        },
      })
      const invokeData = result.data as InvokeAgentData | undefined
      if (invokeData?.invokeAgent) {
        setChatLog((prev) => [...prev, `Support: ${invokeData.invokeAgent.completion}`])
      }
    } catch (err: any) {
      setChatLog((prev) => [...prev, `Support: ${err.message}`])
    }
  }

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="bg-white border-b border-slate-200 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-slate-900 rounded-lg text-white">
            <Package className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-slate-900">Bunker Store</h1>
            <p className="text-xs text-slate-500">Secure infrastructure for AI agents</p>
          </div>
        </div>
        <button
          onClick={onLogout}
          className="p-2 text-slate-400 hover:text-slate-900 transition-colors"
          title="Sign out"
        >
          <LogOut className="w-5 h-5" />
        </button>
      </header>

      <main className="max-w-6xl mx-auto p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          <section>
            <h2 className="text-lg font-bold text-slate-900 mb-4">Products</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {PRODUCTS.map((product) => (
                <div key={product.name} className="bg-white border border-slate-200 rounded-xl p-5">
                  <h3 className="font-bold text-slate-900">{product.name}</h3>
                  <p className="text-sm text-slate-500 mt-1">{product.description}</p>
                  <div className="flex items-center justify-between mt-4">
                    <span className="text-xl font-bold text-slate-900">${product.price}</span>
                    <button
                      onClick={() => addToCart(product.name)}
                      className="bg-slate-900 hover:bg-slate-800 text-white text-sm font-semibold px-4 py-2 rounded-lg transition-colors"
                    >
                      Add to cart
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section>
            <h2 className="text-lg font-bold text-slate-900 mb-4">Your orders</h2>
            {loading ? (
              <div className="text-sm text-slate-500">Loading...</div>
            ) : orders.length === 0 ? (
              <div className="text-sm text-slate-500">No orders yet.</div>
            ) : (
              <div className="space-y-2">
                {orders.map((order) => (
                  <div key={order.id} className="bg-white border border-slate-200 rounded-xl p-4 flex items-center justify-between">
                    <div>
                      <div className="font-semibold text-slate-900">{order.product}</div>
                      <div className="text-xs text-slate-500">Qty {order.quantity} · ${order.totalUsd}</div>
                    </div>
                    <span className={`text-xs font-semibold px-2 py-1 rounded ${
                      order.status === 'delivered' ? 'bg-emerald-100 text-emerald-700' :
                      order.status === 'pending' ? 'bg-amber-100 text-amber-700' :
                      'bg-slate-100 text-slate-700'
                    }`}>
                      {order.status}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>

        <aside className="lg:col-span-1">
          <div className="bg-white border border-slate-200 rounded-xl p-5 sticky top-6">
            <h2 className="font-bold text-slate-900 flex items-center gap-2 mb-4">
              <ShoppingCart className="w-4 h-4" />
              Cart
            </h2>
            {cartItems.length === 0 ? (
              <p className="text-sm text-slate-500">Your cart is empty.</p>
            ) : (
              <>
                <div className="space-y-2 mb-4">
                  {cartItems.map((item) => (
                    <div key={item.name} className="flex items-center justify-between text-sm">
                      <span className="text-slate-700">{item.name} × {item.quantity}</span>
                      <span className="font-semibold text-slate-900">${item.price * item.quantity}</span>
                    </div>
                  ))}
                </div>
                <div className="border-t border-slate-200 pt-3 mb-4 flex items-center justify-between font-bold">
                  <span className="text-slate-900">Total</span>
                  <span className="text-slate-900">${cartTotal}</span>
                </div>
                <button
                  onClick={placeOrder}
                  disabled={creating}
                  className="w-full bg-slate-900 hover:bg-slate-800 text-white font-semibold py-2.5 rounded-lg transition-colors disabled:opacity-50"
                >
                  {creating ? 'Placing order...' : 'Place order'}
                </button>
              </>
            )}
          </div>
        </aside>
      </main>

      {/* Support chat */}
      <button
        onClick={() => setChatOpen(!chatOpen)}
        className="fixed bottom-6 right-6 bg-slate-900 hover:bg-slate-800 text-white p-4 rounded-full shadow-lg transition-colors"
      >
        {chatOpen ? <X className="w-5 h-5" /> : <MessageCircle className="w-5 h-5" />}
      </button>

      {chatOpen && (
        <div className="fixed bottom-24 right-6 w-96 h-96 bg-white border border-slate-200 rounded-2xl shadow-2xl flex flex-col">
          <div className="p-4 border-b border-slate-200">
            <h3 className="font-bold text-slate-900">Support</h3>
            <p className="text-xs text-slate-500">AI agent · order tracking, proofs, refunds</p>
          </div>
          <div className="flex-1 overflow-y-auto p-4 space-y-2 text-sm">
            {chatLog.length === 0 && (
              <p className="text-slate-400 text-xs">Ask about your orders, proofs, or refunds.</p>
            )}
            {chatLog.map((line, i) => (
              <div key={i} className={line.startsWith('You:') ? 'text-slate-900' : 'text-slate-600'}>
                {line}
              </div>
            ))}
          </div>
          <form onSubmit={sendChat} className="p-3 border-t border-slate-200 flex gap-2">
            <input
              value={chatInput}
              onChange={(e) => setChatInput(e.target.value)}
              placeholder="Type a message..."
              disabled={chatting}
              className="flex-1 border border-slate-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-slate-900"
            />
            <button
              type="submit"
              disabled={chatting || !chatInput.trim()}
              className="bg-slate-900 hover:bg-slate-800 text-white px-4 py-2 rounded-lg text-sm font-semibold disabled:opacity-50"
            >
              Send
            </button>
          </form>
        </div>
      )}
    </div>
  )
}