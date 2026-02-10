import { Link } from 'react-router-dom'

export default function NotFound() {
  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-4xl font-bold text-gray-900">404</h1>
        <p className="mt-2 text-gray-600">Page not found.</p>
        <Link to="/" className="mt-4 inline-block text-blue-600 hover:text-blue-800">
          Go to Dashboard
        </Link>
      </div>
    </div>
  )
}
