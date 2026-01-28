import { ChevronRight, ChevronDown, Folder, FileCode, AlertCircle, BarChart3, ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import type { FeatureFlag } from '@/types';
import { format } from 'date-fns';

interface StrategyRendererProps {
    value: string;
    type: string;
}

export function StrategyRenderer({ value, type }: StrategyRendererProps) {
    if (type !== 'strategy') {
        let display = value;
        if (value.length > 50) display = value.substring(0, 50) + '...';
        
        return (
            <code className="bg-secondary px-2 py-0.5 rounded text-xs font-mono text-foreground/80 break-all">
                {display}
            </code>
        );
    }

    try {
        const rules = JSON.parse(value);
        // Simple heuristic for visualization
        // e.g., { "whitelist": [1,2], "percentage": 10 }
        
        return (
            <div className="flex flex-col gap-1">
                {Array.isArray(rules.whitelist) && (
                    <div className="flex items-center gap-1 text-xs text-green-400 bg-green-400/10 px-2 py-0.5 rounded w-fit">
                        <ShieldCheck className="h-3 w-3" />
                        <span>Whitelist: {rules.whitelist.length} users</span>
                    </div>
                )}
                {(rules.percentage !== undefined) && (
                   <div className="w-full max-w-[120px]">
                       <div className="flex justify-between text-[10px] text-muted-foreground mb-0.5">
                           <span>Rollout</span>
                           <span>{rules.percentage}%</span>
                       </div>
                       <div className="h-1.5 w-full bg-secondary rounded-full overflow-hidden">
                           <div className="h-full bg-blue-500 rounded-full" style={{ width: `${rules.percentage}%` }} />
                       </div>
                   </div>
                )}
                 {/* Fallback if structure is unknown */}
                 {!rules.whitelist && rules.percentage === undefined && (
                     <div className="text-xs text-muted-foreground flex items-center gap-1">
                         <FileCode className="h-3 w-3" />
                         <span>Complex Strategy</span>
                     </div>
                 )}
            </div>
        );
    } catch (e) {
        return (
            <div className="flex items-center gap-1 text-xs text-red-500 bg-red-500/10 px-2 py-0.5 rounded">
                <AlertCircle className="h-3 w-3" />
                <span>Invalid JSON</span>
            </div>
        );
    }
}

interface TreeNode {
    name: string;
    fullName: string; // The full dot.notation key leading here
    isLeaf: boolean;
    feature?: FeatureFlag;
    children: Record<string, TreeNode>;
}

interface FeatureTreeProps {
    features: FeatureFlag[];
    onEdit: (f: FeatureFlag) => void;
    onViewAudits: (key: string) => void;
}

export function FeatureTree({ features, onEdit, onViewAudits }: FeatureTreeProps) {
    const [expanded, setExpanded] = useState<Record<string, boolean>>({});

    const toggle = (path: string) => {
        setExpanded(prev => ({ ...prev, [path]: !prev[path] }));
    };

    // Build Tree
    const root: Record<string, TreeNode> = {};
    
    features.forEach(f => {
        const parts = f.key.split('.');
        let currentLevel = root;
        let path = "";
        
        parts.forEach((part, index) => {
            path = path ? `${path}.${part}` : part;
            if (!currentLevel[part]) {
                currentLevel[part] = {
                    name: part,
                    fullName: path,
                    isLeaf: false,
                    children: {}
                };
            }
            
            if (index === parts.length - 1) {
                currentLevel[part].isLeaf = true;
                currentLevel[part].feature = f;
            }
            
            currentLevel = currentLevel[part].children;
        });
    });

    const renderNode = (node: TreeNode, depth: number) => {
        const isExpanded = expanded[node.fullName] || false; // Default collapsed
        const hasChildren = Object.keys(node.children).length > 0;
        
        return (
            <div key={node.fullName}>
                <div 
                    className={`
                        flex items-center border-b border-border hover:bg-secondary/30 transition-colors
                        ${node.isLeaf ? 'h-16' : 'h-10 bg-secondary/10'}
                    `}
                >
                    {/* Tree Indentation & Folder Name */}
                    <div 
                        className="flex-1 flex items-center gap-2 px-4 overflow-hidden" 
                        style={{ paddingLeft: `${depth * 1.5 + 1}rem` }}
                    >
                        {hasChildren ? (
                            <button onClick={() => toggle(node.fullName)} className="p-1 hover:bg-secondary rounded">
                                {isExpanded ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                            </button>
                        ) : <div className="w-6" />}
                        
                        {node.isLeaf ? (
                            <div className="flex flex-col">
                                <span className="font-medium text-sm text-foreground">{node.name}</span>
                                <span className="text-xs text-muted-foreground">{node.fullName}</span>
                            </div>
                        ) : (
                            <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground cursor-pointer select-none" onClick={() => toggle(node.fullName)}>
                                <Folder className="h-4 w-4" />
                                {node.name}
                            </div>
                        )}
                    </div>

                    {/* Columns (Only for Leaf Nodes) */}
                    {node.isLeaf && node.feature ? (
                        <>
                             {/* Value / Strategy */}
                            <div className="w-1/4 px-4">
                                <StrategyRenderer value={node.feature.value} type={node.feature.type} />
                            </div>
                            
                            {/* Dual Clock */}
                            <div className="w-48 px-4 flex flex-col justify-center text-xs">
                                <div className="flex items-center gap-2 mb-1">
                                    <span className="bg-primary/10 text-primary px-1.5 py-0.5 rounded font-mono font-bold">v{node.feature.version}</span>
                                    {/* Mocking Rev for now as backend doesn't provide it yet */}
                                    <span className="text-muted-foreground/50 font-mono">rev:{(node.feature.id * 1024).toString(16)}</span>
                                </div>
                                <div className="text-muted-foreground">
                                    by <span className="text-foreground">{node.feature.updated_by || 'system'}</span>
                                </div>
                                <div className="text-muted-foreground/70 scale-90 origin-left">
                                     {node.feature.updated_at ? format(new Date(node.feature.updated_at), 'MMM d, HH:mm') : '-'}
                                </div>
                            </div>
                            
                            {/* Actions */}
                             <div className="w-32 px-4 flex justify-end gap-2">
                                <button 
                                    onClick={() => onViewAudits(node.feature!.key)}
                                    className="p-2 text-muted-foreground hover:text-foreground hover:bg-secondary rounded-md" 
                                    title="Audit Trail"
                                >
                                    <ShieldCheck className="h-4 w-4" />
                                </button>
                                <button 
                                    onClick={() => onEdit(node.feature!)}
                                    className="p-2 text-muted-foreground hover:text-primary hover:bg-secondary rounded-md"
                                    title="Edit"
                                >
                                    <FileCode className="h-4 w-4" />
                                </button>
                            </div>
                        </>
                    ) : (
                         <div className="w-[calc(25%+20rem)]" /> // Spacer for folder rows
                    )}
                </div>
                
                {/* Recursion */}
                {isExpanded && hasChildren && (
                    <div>
                        {Object.values(node.children)
                            .sort((a, b) => a.isLeaf === b.isLeaf ? a.name.localeCompare(b.name) : (a.isLeaf ? 1 : -1)) // Folders first
                            .map(child => renderNode(child, depth + 1))
                        }
                    </div>
                )}
            </div>
        );
    };

    return (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
             {/* Header */}
             <div className="flex items-center bg-secondary/50 text-muted-foreground font-medium text-xs h-10 border-b border-border">
                <div className="flex-1 px-4 pl-10">Feature Key / Hierarchy</div>
                <div className="w-1/4 px-4">Value / Strategy</div>
                <div className="w-48 px-4">Version & Metadata</div>
                <div className="w-32 px-4 text-right">Actions</div>
             </div>
             
             {/* Body */}
             <div className="divide-y divide-border">
                {Object.values(root)
                    .sort((a, b) => a.isLeaf === b.isLeaf ? a.name.localeCompare(b.name) : (a.isLeaf ? 1 : -1))
                    .map(node => renderNode(node, 0))
                }
             </div>
        </div>
    );
}
