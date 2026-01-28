import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { featureService } from '@/services/feature';
import { authService } from '@/services/auth';
import type { FeatureFlag, FeatureAudit } from '@/types';
import FeatureDrawer from '@/components/FeatureDrawer';
import AuditLog from '@/components/AuditLog';
import RealtimePanel from '@/components/RealtimePanel';
import { FeatureTree } from '@/components/FeatureTree';
import { 
  LogOut, 
  Settings, 
  Search, 
  Plus, 
  RefreshCw,
  Globe,
  Database
} from 'lucide-react';

export default function Dashboard() {
  const [features, setFeatures] = useState<FeatureFlag[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedFeature, setSelectedFeature] = useState<FeatureFlag | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  
  // Context State
  const [env, setEnv] = useState('dev');
  const [namespace, setNamespace] = useState('default');
  
  // Audit View State
  const [showAudits, setShowAudits] = useState(false);
  const [auditFeatureKey, setAuditFeatureKey] = useState('');
  const [audits, setAudits] = useState<FeatureAudit[]>([]);

  const navigate = useNavigate();

  const loadFeatures = async () => {
    setLoading(true);
    try {
      const data = await featureService.getAll(namespace, env);
      setFeatures(data);
    } catch (error) {
      console.error('Failed to load features', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFeatures();
  }, [env, namespace]);

  const handleLogout = async () => {
    await authService.logout();
    navigate('/login');
  };

  const handleEdit = (feature: FeatureFlag) => {
      setSelectedFeature(feature);
      setIsDrawerOpen(true);
  };
  
  const handleAddNew = () => {
      setSelectedFeature(null);
      setIsDrawerOpen(true);
  };

  const handleViewAudits = async (key: string) => {
      setAuditFeatureKey(key);
      try {
          const data = await featureService.getAudits(key);
          setAudits(data);
          setShowAudits(true);
      } catch (e) {
          alert('Failed to load audits');
      }
  };

  const handleRollback = async (auditId: number) => {
      if(!confirm('Are you sure you want to rollback to this version?')) return;
      try {
          await featureService.rollback(auditFeatureKey, auditId, namespace, env);
          alert('Rollback successful');
          setShowAudits(false);
          loadFeatures();
      } catch (e) {
          alert('Rollback failed');
      }
  };

  return (
    <div className="min-h-screen bg-background text-foreground flex">
      {/* Sidebar */}
      <aside className="w-64 border-r border-border flex flex-col pt-6 pb-4 px-4 bg-zinc-950/20">
        <div className="flex items-center gap-3 px-2 mb-8">
            <div className="h-8 w-8 rounded bg-primary flex items-center justify-center text-primary-foreground font-bold shadow-lg shadow-primary/20">M</div>
            <span className="font-bold text-xl tracking-tight">MizuFlow</span>
        </div>
        
        <nav className="flex-1 space-y-1">
            <a href="#" className="flex items-center gap-3 px-3 py-2 text-sm font-medium bg-primary/10 text-primary rounded-md">
                <Settings className="h-4 w-4" /> Feature Flags
            </a>
            {/* Additional nav items... */}
        </nav>

        <div className="mt-auto border-t border-border pt-4">
            <button 
                onClick={handleLogout}
                className="flex w-full items-center gap-3 px-3 py-2 text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-secondary rounded-md"
            >
                <LogOut className="h-4 w-4" /> Sign out
            </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-screen overflow-hidden relative">
        <header className="h-16 border-b border-border flex items-center justify-between px-8 bg-background/50 backdrop-blur-sm z-10">
            {/* Global Context Switcher */}
            <div className="flex items-center gap-6">
                 {/* Environment Selector */}
                 <div className="flex items-center gap-2">
                     <Globe className="h-4 w-4 text-muted-foreground" />
                     <select 
                        value={env}
                        onChange={(e) => setEnv(e.target.value)}
                        className="bg-transparent text-sm font-medium focus:outline-none cursor-pointer hover:text-primary transition-colors"
                     >
                         <option value="dev">Environment: Dev</option>
                         <option value="test">Environment: Test</option>
                         <option value="prod">Environment: Prod</option>
                     </select>
                 </div>
                 
                 <div className="h-4 w-[1px] bg-border" />
                 
                 {/* Namespace Selector */}
                 <div className="flex items-center gap-2">
                     <Database className="h-4 w-4 text-muted-foreground" />
                     <select 
                        value={namespace}
                        onChange={(e) => setNamespace(e.target.value)}
                        className="bg-transparent text-sm font-medium focus:outline-none cursor-pointer hover:text-primary transition-colors"
                     >
                         <option value="default">Namespace: Default</option>
                         <option value="system">Namespace: System</option>
                         <option value="marketing">Namespace: Marketing</option>
                         <option value="order">Namespace: Order</option>
                     </select>
                 </div>
            </div>

            <div className="flex items-center gap-4">
                <button 
                    onClick={handleAddNew}
                    className="flex items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-all shadow-sm hover:shadow-md hover:shadow-primary/20"
                >
                    <Plus className="h-4 w-4" /> Create Feature
                </button>
            </div>
        </header>

        <div className="p-8 flex-1 overflow-auto bg-dot-pattern">
            {/* Filters & Search */}
            <div className="flex items-center gap-4 mb-6">
                <div className="relative flex-1 max-w-md">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <input 
                        className="w-full bg-background border border-border rounded-md pl-9 pr-4 py-2 text-sm focus:ring-1 focus:ring-primary outline-none shadow-sm" 
                        placeholder="Search feature keys (e.g. search.ui.enabled)..." 
                    />
                </div>
                <button onClick={loadFeatures} className="p-2 text-muted-foreground hover:text-foreground bg-card border border-border rounded-md shadow-sm">
                    <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                </button>
            </div>

            {/* Tree View Table */}
            {loading ? (
                <div className="flex items-center justify-center h-64 text-muted-foreground flex-col gap-4">
                    <div className="h-8 w-8 rounded-full border-2 border-primary border-t-transparent animate-spin" />
                    Loading features...
                </div>
            ) : (
                <FeatureTree features={features} onEdit={handleEdit} onViewAudits={handleViewAudits} />
            )}
        </div>
        
        {/* Drawers & Overlays */}
        {showAudits && (
            <>
                <div className="fixed inset-0 bg-background/50 backdrop-blur-sm z-40 transition-opacity" onClick={() => setShowAudits(false)} />
                <AuditLog 
                    audits={audits}
                    featureKey={auditFeatureKey}
                    onRollback={handleRollback}
                    onClose={() => setShowAudits(false)}
                />
            </>
        )}
      </main>

      <FeatureDrawer 
        isOpen={isDrawerOpen} 
        onClose={() => setIsDrawerOpen(false)}
        feature={selectedFeature}
        onSave={() => loadFeatures()}
      />

      <RealtimePanel />
    </div>
  );
}
