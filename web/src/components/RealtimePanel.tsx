import { useEffect, useState, useRef } from 'react';
import { Activity, Minus, Maximize2, Minimize2, Square, MessageSquare, GripHorizontal } from 'lucide-react'; // Added icons
import { authService } from '@/services/auth';

interface StreamMessage {
    type: string;
    key: string;
    namespace: string;
    env: string;
    version: number;
    value: string;
    updated_at: number;
}

export default function RealtimePanel() {
  const [events, setEvents] = useState<string[]>([]);
  const [status, setStatus] = useState<'connected' | 'disconnected' | 'connecting'>('disconnected');
  
  // UI State
  const [isMinimized, setIsMinimized] = useState(true);
  const [isMaximized, setIsMaximized] = useState(false);
  const [position, setPosition] = useState<{x: number, y: number} | null>(null);
  const [size, setSize] = useState<{width: number, height: number} | null>(null); // New Size State
  
  const panelRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ startX: number, startY: number, initialLeft: number, initialTop: number } | null>(null);
  const resizeRef = useRef<{ startX: number, startY: number, initialWidth: number, initialHeight: number } | null>(null);

  useEffect(() => {
    if (!authService.isAuthenticated()) return;
// ... (rest of useEffect logic remains same)
    const token = localStorage.getItem('access_token');
    const eventSource = new EventSource(`/v1/admin/stream?token=${token}&env=dev`);

    eventSource.onopen = () => {
        setStatus('connected');
        setEvents(prev => [`[${new Date().toLocaleTimeString()}] System: Connected to stream`, ...prev]);
    };

    eventSource.addEventListener('ping', () => {
        // Heartbeat received
    });

    eventSource.onmessage = (event) => {
        try {
            const data: StreamMessage = JSON.parse(event.data);
            // Use backend timestamp if available, otherwise fallback to local receipt time
            const timeDate = data.updated_at ? new Date(data.updated_at) : new Date();
            const timestamp = timeDate.toLocaleTimeString([], { hour12: false });
            
            const log = `[${timestamp}] [${data.env}/${data.namespace}] ${data.key} updated to ${data.value}`;
            
            setEvents(prev => [log, ...prev.slice(0, 49)]); // Increased buffer
        } catch (e) {
            console.error('Failed to parse SSE', e);
        }
    };

    eventSource.onerror = () => {
        if (eventSource.readyState === EventSource.CLOSED) {
            setStatus('disconnected');
        } else {
            setStatus('disconnected');
        }
    };

    return () => {
      eventSource.close();
    };
  }, []);

  // Drag Logic
  const handleMouseDown = (e: React.MouseEvent) => {
    if (isMaximized) return;
    // Only allow left click
    if (e.button !== 0) return;
    
    // Prevent default to avoid text selection inside
    if (!(e.target as HTMLElement).closest('button') && !(e.target as HTMLElement).closest('.resize-handle')) {
        e.preventDefault();
    } else {
        return; // Don't drag if clicking buttons or resize handle
    }

    const rect = panelRef.current?.getBoundingClientRect();
    if (!rect) return;

    // Initialize position on first drag
    const currentX = position ? position.x : rect.left;
    const currentY = position ? position.y : rect.top;

    if (!position) {
        setPosition({ x: rect.left, y: rect.top });
    }
    
    // Also init size on first move to prevent jumping
    if (!size) {
        setSize({ width: rect.width, height: rect.height });
    }

    dragRef.current = {
        startX: e.clientX,
        startY: e.clientY,
        initialLeft: currentX,
        initialTop: currentY,
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  };

  const handleMouseMove = (e: MouseEvent) => {
      if (!dragRef.current) return;
      const dx = e.clientX - dragRef.current.startX;
      const dy = e.clientY - dragRef.current.startY;
      
      setPosition({
          x: dragRef.current.initialLeft + dx,
          y: dragRef.current.initialTop + dy
      });
  };

  const handleMouseUp = () => {
      dragRef.current = null;
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
  };

  // Resize Logic
  const handleResizeMouseDown = (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation(); // Prevent drag start

      const rect = panelRef.current?.getBoundingClientRect();
      if (!rect) return;

      if (!size) {
         setSize({ width: rect.width, height: rect.height });
      }
      
      // If position is not set, set it now so it snaps to absolute
      if (!position) {
          setPosition({ x: rect.left, y: rect.top });
      }

      resizeRef.current = {
          startX: e.clientX,
          startY: e.clientY,
          initialWidth: rect.width,
          initialHeight: rect.height
      };
      
      document.addEventListener('mousemove', handleResizeMove);
      document.addEventListener('mouseup', handleResizeUp);
  };

  const handleResizeMove = (e: MouseEvent) => {
      if (!resizeRef.current) return;
      const dx = e.clientX - resizeRef.current.startX;
      const dy = e.clientY - resizeRef.current.startY;
      
      const newWidth = Math.max(300, resizeRef.current.initialWidth + dx); // Min width 300
      const newHeight = Math.max(100, resizeRef.current.initialHeight + dy); // Min height 100

      setSize({ width: newWidth, height: newHeight });
  };
  
  const handleResizeUp = () => {
    resizeRef.current = null;
    document.removeEventListener('mousemove', handleResizeMove);
    document.removeEventListener('mouseup', handleResizeUp);
  };


  // Dynamic Styles
  const getStyle = () => {
      if (isMaximized) {
          return { inset: 0, width: '100%', height: '100%' };
      }
      const style: React.CSSProperties = {};
      if (position) {
          style.top = position.y;
          style.left = position.x;
      }
      if (size && !isMinimized) {
          style.width = size.width;
          style.height = size.height;
      } else if (!size && position) {
           style.width = '24rem'; // Default width w-96
      }
      return style;
  };

  return (
    <div 
        ref={panelRef}
        style={getStyle()}
        className={`fixed bg-card border border-border shadow-xl rounded-lg overflow-hidden flex flex-col z-[100] transition-shadow duration-200
            ${!position && !isMaximized ? 'bottom-6 left-72 w-96' : ''} 
            ${isMinimized && !isMaximized ? 'w-auto h-auto !width-auto' : ''}
        `}
    >
      <div 
        onMouseDown={handleMouseDown}
        className={`bg-secondary px-4 py-2 flex items-center justify-between border-b border-border select-none
            ${isMaximized ? '' : 'cursor-move'}
        `}
      >
        <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-green-500 animate-pulse" />
            <span className="text-sm font-semibold whitespace-nowrap">Stream</span>
            <span className={`h-2 w-2 rounded-full ${status === 'connected' ? 'bg-green-500' : 'bg-red-500'}`} />
        </div>
        
        <div className="flex items-center gap-1 ml-4">
            <button 
                onClick={() => setIsMinimized(!isMinimized)} 
                className="p-1 hover:bg-white/10 rounded"
                title={isMinimized ? "Expand" : "Minimize"}
            >
                {isMinimized ? <MessageSquare className="h-4 w-4" /> : <Minus className="h-4 w-4" />}
            </button>
            {!isMinimized && (
                <button 
                    onClick={() => {
                        setIsMaximized(!isMaximized);
                        // Reset position if restoring from max to ensure it doesn't fly away if logic was weird, 
                        // though we usually keep position for restore.
                    }} 
                    className="p-1 hover:bg-white/10 rounded"
                    title={isMaximized ? "Restore" : "Maximize"}
                >
                     {isMaximized ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
                </button>
            )}
        </div>
      </div>
      
      {!isMinimized && (
          <div className={`p-2 overflow-y-auto bg-black text-xs font-mono text-green-400 space-y-1 font-semibold ${(isMaximized || size) ? 'flex-1' : 'h-40'} ${isMaximized ? 'p-4 text-sm' : ''}`}>
            {events.length === 0 && <span className="opacity-50">Waiting for events...</span>}
            {events.map((event, i) => (
                <div key={i} className="break-all border-b border-green-900/30 pb-0.5 mb-0.5 last:border-0">{event}</div>
            ))}
          </div>
      )}
      
      {/* Resize Handle */}
      {!isMinimized && !isMaximized && (
          <div 
            className="absolute bottom-0 right-0 p-1 cursor-se-resize text-muted-foreground/50 hover:text-foreground resize-handle"
            onMouseDown={handleResizeMouseDown}
          >
              <GripHorizontal className="h-4 w-4 rotate-45" />
          </div>
      )}
    </div>
  );
}
