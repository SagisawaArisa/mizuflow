import { format } from 'date-fns';
import type { FeatureAudit } from '@/types';
import { ArrowLeft, Clock } from 'lucide-react';

interface AuditLogProps {
  audits: FeatureAudit[];
  onRollback: (auditId: number) => void;
  onClose: () => void;
  featureKey: string;
}

export default function AuditLog({ audits, onRollback, onClose, featureKey }: AuditLogProps) {
  return (
    <div className="fixed inset-y-0 right-0 w-[40rem] z-50 bg-background border-l border-border shadow-2xl flex flex-col animate-in slide-in-from-right duration-300">
        <div className="flex-none p-6 border-b border-border flex items-center justify-between bg-secondary/30">
             <div>
                <h2 className="text-xl font-bold flex items-center gap-2">
                    <Clock className="h-5 w-5 text-primary" /> 
                    Audit Trail
                </h2>
                <div className="text-sm text-muted-foreground font-mono mt-1">{featureKey}</div>
             </div>
             <button onClick={onClose} className="p-2 hover:bg-white/10 rounded-full transition-colors">
                 <ArrowLeft className="h-5 w-5" />
             </button>
        </div>

      <div className="flex-1 overflow-y-auto relative border-l-2 border-border ml-8 my-6 space-y-8 pr-6">
        {audits.map((audit) => (
          <div key={audit.id} className="ml-8 relative group">
              {/* Timeline Dot */}
            <span className={`
                absolute -left-[41px] top-6 h-5 w-5 rounded-full border-4 border-background 
                ${audit.type === 'create' ? 'bg-green-500' : 'bg-blue-500'}
                group-hover:scale-110 transition-transform shadow-sm
            `} />
            
            <div className="bg-card p-5 rounded-xl border border-border/50 shadow-sm hover:shadow-md transition-shadow">
                <div className="flex justify-between items-start mb-2">
                    <div>
                        <span className="text-xs font-mono bg-primary/10 text-primary px-2 py-0.5 rounded uppercase">{audit.type}</span>
                        <span className="text-muted-foreground text-sm ml-2">by {audit.operator}</span>
                    </div>
                    <time className="text-xs text-muted-foreground">{format(new Date(audit.created_at), 'MMM d, yyyy HH:mm:ss')}</time>
                </div>
                
                <div className="grid grid-cols-2 gap-4 text-sm mt-3">
                    <div className="bg-red-950/20 p-2 rounded border border-red-900/20">
                        <div className="text-xs text-red-400 mb-1">Old Value</div>
                        <code className="break-all">{audit.old_value || '(empty)'}</code>
                    </div>
                    <div className="bg-green-950/20 p-2 rounded border border-green-900/20">
                        <div className="text-xs text-green-400 mb-1">New Value</div>
                         <code className="break-all">{audit.new_value}</code>
                    </div>
                </div>
                
                <div className="mt-3 flex justify-end">
                    <button 
                        onClick={() => onRollback(audit.id)}
                        className="text-xs text-primary hover:underline"
                    >
                        Rollback to this version
                    </button>
                </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
