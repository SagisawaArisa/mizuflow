import { useState, useEffect } from 'react';
import type { FeatureFlag } from '@/types';
import { X, Save, AlertTriangle } from 'lucide-react';
import { featureService } from '@/services/feature';

interface FeatureDrawerProps {
  feature: FeatureFlag | null;
  isOpen: boolean;
  onClose: () => void;
  onSave: () => void;
}

export default function FeatureDrawer({ feature, isOpen, onClose, onSave }: FeatureDrawerProps) {
  const [formData, setFormData] = useState<Partial<FeatureFlag>>({});
  const [jsonError, setJsonError] = useState('');

  useEffect(() => {
    if (feature) {
      setFormData({ ...feature });
    } else {
        setFormData({
            namespace: 'default',
            env: 'dev',
            type: 'bool',
            value: 'false'
        });
    }
  }, [feature, isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.key || !formData.value) return;

    try {
      await featureService.create(formData as FeatureFlag);
      onSave();
      onClose();
    } catch (err) {
      console.error(err);
      alert('Failed to save feature');
    }
  };

  const handleValueChange = (val: string) => {
     setFormData({ ...formData, value: val });
     if (formData.type === 'json' || formData.type === 'strategy') {
         try {
             JSON.parse(val);
             setJsonError('');
         } catch (e) {
             setJsonError('Invalid JSON format');
         }
     }
  };

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-background/80 backdrop-blur-sm" onClick={onClose} />

      {/* Drawer */}
      <div className="relative z-50 w-full max-w-lg h-full bg-card border-l border-border shadow-2xl flex flex-col">
        <div className="p-4 border-b border-border flex items-center justify-between">
          <h2 className="text-lg font-semibold">{feature ? 'Edit Feature' : 'New Feature'}</h2>
          <button onClick={onClose} className="p-2 hover:bg-secondary rounded-full">
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6 overflow-y-auto flex-1">
          <div>
            <label className="block text-sm font-medium mb-1">Key {feature && <span className="text-xs text-muted-foreground">(Read-only)</span>}</label>
            <input
              disabled={!!feature}
              type="text"
              value={formData.key || ''}
              onChange={(e) => setFormData({...formData, key: e.target.value})}
              className="w-full rounded-md bg-secondary border-transparent focus:border-primary focus:bg-background px-3 py-2 disabled:opacity-50"
              placeholder="e.g. new-ui-enabled"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
             <div>
                <label className="block text-sm font-medium mb-1">Namespace</label>
                <input
                    disabled={!!feature}
                    value={formData.namespace || 'default'}
                    onChange={(e) => setFormData({...formData, namespace: e.target.value})}
                    className="w-full rounded-md bg-secondary border-transparent px-3 py-2 disabled:opacity-50"
                />
             </div>
             <div>
                <label className="block text-sm font-medium mb-1">Env</label>
                <input
                     disabled={!!feature}
                    value={formData.env || 'dev'}
                    onChange={(e) => setFormData({...formData, env: e.target.value})}
                    className="w-full rounded-md bg-secondary border-transparent px-3 py-2 disabled:opacity-50"
                />
             </div>
          </div>

          <div>
             <label className="block text-sm font-medium mb-1">Type</label>
             <select
                 value={formData.type || 'bool'}
                 onChange={(e) => setFormData({...formData, type: e.target.value as any})}
                 className="w-full rounded-md bg-secondary border-transparent px-3 py-2"
                 disabled={!!feature} 
             >
                 <option value="bool">Boolean</option>
                 <option value="string">String</option>
                 <option value="json">JSON</option>
                 <option value="strategy">Strategy</option>
             </select>
          </div>

          <div>
            <label className="block text-sm font-medium mb-3">Value / Strategy</label>
            
            {formData.type === 'bool' ? (
                 <div className="flex items-center gap-3">
                    <span className={formData.value === 'false' ? 'font-bold' : 'text-muted-foreground'}>False</span>
                    <button
                        type="button"
                        onClick={() => handleValueChange(formData.value === 'true' ? 'false' : 'true')}
                        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${formData.value === 'true' ? 'bg-primary' : 'bg-secondary'}`}
                    >
                        <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${formData.value === 'true' ? 'translate-x-6' : 'translate-x-1'}`} />
                    </button>
                    <span className={formData.value === 'true' ? 'font-bold' : 'text-muted-foreground'}>True</span>
                 </div>
            ) : (
                <div className="relative">
                    <textarea 
                        rows={10}
                        value={formData.value || ''}
                        onChange={(e) => handleValueChange(e.target.value)}
                        className={`w-full font-mono text-sm bg-black/80 text-green-400 p-4 rounded-md border ${jsonError ? 'border-red-500' : 'border-transparent'}`}
                    />
                    {jsonError && (
                        <div className="absolute bottom-4 right-4 text-red-500 text-xs flex items-center bg-black/50 px-2 py-1 rounded">
                             <AlertTriangle className="h-3 w-3 mr-1" /> {jsonError}
                        </div>
                    )}
                </div>
            )}
          </div>
        </form>

        <div className="p-4 border-t border-border bg-secondary/50">
            <button
                onClick={handleSubmit} 
                className="w-full flex justify-center items-center gap-2 bg-primary text-primary-foreground hover:bg-primary/90 py-2 rounded-md font-semibold transition-all"
            >
                <Save className="h-4 w-4" /> Save Changes
            </button>
        </div>
      </div>
    </div>
  );
}

