import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from '@/pages/Login';
import Dashboard from '@/pages/Dashboard';
import { authService } from '@/services/auth';

const PrivateRoute = ({ children }: { children: React.ReactNode }) => {
  // Simple check for now. For better UX, verify token validity or use a context.
  const isAuth = authService.isAuthenticated();
  if (!isAuth) {
      return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
};

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/*"
          element={
            <PrivateRoute>
              <Dashboard />
            </PrivateRoute>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}

export default App;

