'use client';

import { useState, useEffect } from 'react';
import { useLanguage } from '@/contexts/LanguageContext';

export type ColumnId = 'index' | 'problemTitle' | 'time' | 'answer' | 'ratio' | 'hot1' | 'comment' | 'detail';

export interface ColumnConfig {
  id: ColumnId;
  visible: boolean;
  order: number;
}

const DEFAULT_COLUMNS: ColumnConfig[] = [
  { id: 'index', visible: true, order: 0 },
  { id: 'problemTitle', visible: true, order: 1 },
  { id: 'time', visible: true, order: 2 },
  { id: 'answer', visible: true, order: 3 },
  { id: 'ratio', visible: true, order: 4 },
  { id: 'hot1', visible: false, order: 5 },
  { id: 'comment', visible: false, order: 6 },
  { id: 'detail', visible: true, order: 7 },
];

interface ColumnCustomizerProps {
  isOpen: boolean;
  onClose: () => void;
  columns: ColumnConfig[];
  onChange: (columns: ColumnConfig[]) => void;
  resultLimit: number;
  onResultLimitChange: (limit: number) => void;
}

export default function ColumnCustomizer({ isOpen, onClose, columns, onChange, resultLimit, onResultLimitChange }: ColumnCustomizerProps) {
  const { t } = useLanguage();
  const [localColumns, setLocalColumns] = useState<ColumnConfig[]>(columns);
  const [localResultLimit, setLocalResultLimit] = useState(resultLimit);
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);

  useEffect(() => {
    if (isOpen) {
      setLocalColumns(columns);
      setLocalResultLimit(resultLimit);
    }
  }, [isOpen, columns, resultLimit]);

  const getColumnLabel = (id: ColumnId): string => {
    const labels: Record<ColumnId, string> = {
      index: t('problems.search.index'),
      problemTitle: t('problems.search.problemTitle'),
      time: t('problems.search.time'),
      answer: t('problems.search.answer'),
      ratio: t('problems.search.ratio'),
      hot1: t('problems.search.hot1'),
      comment: t('problems.search.comment'),
      detail: t('problems.search.detail'),
    };
    return labels[id];
  };

  const handleToggle = (id: ColumnId) => {
    if (id === 'index' || id === 'detail') {
      return; // Cannot hide index or detail
    }
    setLocalColumns(prev =>
      prev.map(col => col.id === id ? { ...col, visible: !col.visible } : col)
    );
  };

  const handleDragStart = (index: number) => {
    setDraggedIndex(index);
  };

  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    if (draggedIndex === null) return;

    const item = localColumns[draggedIndex];
    if (item.id === 'index' || item.id === 'detail') return; // Cannot move index or detail

    const targetItem = localColumns[index];
    if (targetItem.id === 'index' || targetItem.id === 'detail') return; // Cannot move to index or detail position

    // Get reorderable columns (excluding index and detail)
    const reorderable = localColumns.filter(col => col.id !== 'index' && col.id !== 'detail');
    const indexCol = localColumns.find(col => col.id === 'index')!;
    const detailCol = localColumns.find(col => col.id === 'detail')!;

    // Find positions in reorderable array
    const draggedPos = reorderable.findIndex(col => col.id === item.id);
    const targetPos = reorderable.findIndex(col => col.id === targetItem.id);

    if (draggedPos === -1 || targetPos === -1) return;

    // Reorder within reorderable columns
    const newReorderable = [...reorderable];
    const [removed] = newReorderable.splice(draggedPos, 1);
    newReorderable.splice(targetPos, 0, removed);

    // Rebuild full array with index first, reorderable in middle, detail last
    const reordered = [
      { ...indexCol, order: 0 },
      ...newReorderable.map((col, idx) => ({ ...col, order: idx + 1 })),
      { ...detailCol, order: newReorderable.length + 1 },
    ];

    setLocalColumns(reordered);
    setDraggedIndex(index);
  };

  const handleDragEnd = () => {
    setDraggedIndex(null);
  };

  const handleSave = () => {
    // Ensure index is first and detail is last
    const sorted = [...localColumns].sort((a, b) => {
      if (a.id === 'index') return -1;
      if (b.id === 'index') return 1;
      if (a.id === 'detail') return 1;
      if (b.id === 'detail') return -1;
      return a.order - b.order;
    });
    
    // Reorder to ensure proper sequence
    const reordered = sorted.map((col, idx) => ({
      ...col,
      order: idx,
    }));
    
    onChange(reordered);
    onResultLimitChange(localResultLimit);
    onClose();
  };

  const handleReset = () => {
    setLocalColumns(DEFAULT_COLUMNS);
    setLocalResultLimit(15);
    onChange(DEFAULT_COLUMNS);
    onResultLimitChange(15);
  };

  if (!isOpen) return null;

  // Filter out index and detail for reordering (they stay fixed)
  const reorderableColumns = localColumns.filter(col => col.id !== 'index' && col.id !== 'detail');
  const indexColumn = localColumns.find(col => col.id === 'index');
  const detailColumn = localColumns.find(col => col.id === 'detail');

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[80vh] overflow-hidden flex flex-col">
        <div className="p-6 border-b">
          <h2 className="text-xl font-bold text-gray-900">{t('problems.search.customizeTitle')}</h2>
          <p className="text-sm text-gray-600 mt-1">{t('problems.search.customizeDescription')}</p>
        </div>

        <div className="p-6 overflow-y-auto flex-1">
          <div className="space-y-2">
            {/* Index column (fixed) */}
            {indexColumn && (
              <div className="flex items-center p-3 bg-gray-50 rounded-lg border border-gray-200">
                <div className="flex-1 flex items-center">
                  <span className="text-sm font-medium text-gray-700">{getColumnLabel(indexColumn.id)}</span>
                  <span className="ml-2 text-xs text-gray-500">(固定)</span>
                </div>
                <div className="flex items-center space-x-2">
                  <span className="text-xs text-gray-500">始终显示</span>
                </div>
              </div>
            )}

            {/* Reorderable columns */}
            {reorderableColumns.map((column, reorderableIndex) => {
              const actualIndex = localColumns.findIndex(c => c.id === column.id);
              const isDragged = draggedIndex === actualIndex;
              return (
                <div
                  key={column.id}
                  draggable={true}
                  onDragStart={() => handleDragStart(actualIndex)}
                  onDragOver={(e) => handleDragOver(e, actualIndex)}
                  onDragEnd={handleDragEnd}
                  className={`flex items-center p-3 rounded-lg border cursor-move transition ${
                    isDragged
                      ? 'bg-indigo-50 border-indigo-300 opacity-50'
                      : 'bg-white border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="flex items-center mr-3 text-gray-400">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8h16M4 16h16" />
                    </svg>
                  </div>
                  <div className="flex-1">
                    <span className="text-sm font-medium text-gray-700">{getColumnLabel(column.id)}</span>
                  </div>
                  <label className="flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={column.visible}
                      onChange={() => handleToggle(column.id)}
                      className="w-4 h-4 text-indigo-600 border-gray-300 rounded focus:ring-indigo-500"
                    />
                    <span className="ml-2 text-sm text-gray-700">
                      {column.visible ? '显示' : '隐藏'}
                    </span>
                  </label>
                </div>
              );
            })}

            {/* Detail column (fixed) */}
            {detailColumn && (
              <div className="flex items-center p-3 bg-gray-50 rounded-lg border border-gray-200">
                <div className="flex-1 flex items-center">
                  <span className="text-sm font-medium text-gray-700">{getColumnLabel(detailColumn.id)}</span>
                  <span className="ml-2 text-xs text-gray-500">(固定)</span>
                </div>
                <div className="flex items-center space-x-2">
                  <span className="text-xs text-gray-500">始终显示</span>
                </div>
              </div>
            )}
          </div>

          {/* Result Limit Control */}
          <div className="mt-6 pt-4 border-t border-gray-200">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-sm font-medium text-gray-700">{t('problems.search.resultLimit')}</span>
                <p className="text-xs text-gray-500 mt-0.5">{t('problems.search.resultLimitDesc')}</p>
              </div>
              <div className="flex items-center space-x-3">
                <input
                  type="range"
                  min="5"
                  max="20"
                  value={localResultLimit}
                  onChange={(e) => setLocalResultLimit(Number(e.target.value))}
                  className="w-24 h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-indigo-600"
                />
                <span className="text-sm font-medium text-gray-900 w-8 text-center">{localResultLimit}</span>
              </div>
            </div>
          </div>
        </div>

        <div className="p-6 border-t flex justify-between">
          <button
            onClick={handleReset}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition"
          >
            {t('problems.search.customizeReset')}
          </button>
          <div className="space-x-3">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition"
            >
              {t('problems.search.customizeClose')}
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition"
            >
              保存
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export { DEFAULT_COLUMNS };
