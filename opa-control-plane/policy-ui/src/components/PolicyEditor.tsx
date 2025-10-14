import { useEffect, useState } from 'react';
import Editor from '@monaco-editor/react';
import type { Policy } from '../types/policy';


interface PolicyEditorProps {
  policy: Policy;
  onSave: (policy: Policy) => void;
  onCancel: () => void;
}

export default function PolicyEditor({ policy, onSave, onCancel }: PolicyEditorProps) {
  const [editedPolicy, setEditedPolicy] = useState<Policy>(policy);

  useEffect(() => {
    setEditedPolicy(policy);
  }, [policy]);

  const handleSave = () => {
    onSave(editedPolicy);
  };

  return (
    <div className="bg-white shadow rounded-lg">
      <div className="p-4 border-b border-gray-200">
        <div className="flex justify-between items-center">
          <h2 className="text-lg font-medium">
            {policy.id ? 'Edit Policy' : 'New Policy'}
          </h2>
          <div className="space-x-3">
            <button
              onClick={onCancel}
              className="px-3 py-2 border border-gray-300 text-sm font-medium rounded-md hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="px-3 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700"
            >
              Save
            </button>
          </div>
        </div>
      </div>
      <div className="p-4 space-y-4">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700">
            Name
          </label>
          <input
            type="text"
            id="name"
            value={editedPolicy.name}
            onChange={(e) => setEditedPolicy({ ...editedPolicy, name: e.target.value })}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
          />
        </div>
        <div>
          <label htmlFor="path" className="block text-sm font-medium text-gray-700">
            Path
          </label>
          <input
            type="text"
            id="path"
            value={editedPolicy.path}
            onChange={(e) => setEditedPolicy({ ...editedPolicy, path: e.target.value })}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
          />
        </div>
        <div>
          <label htmlFor="content" className="block text-sm font-medium text-gray-700">
            Content
          </label>
          <div className="mt-1 border border-gray-300 rounded-md">
            <Editor
              height="400px"
              defaultLanguage="rego"
              value={editedPolicy.content}
              onChange={(value) => setEditedPolicy({ ...editedPolicy, content: value || '' })}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 14,
              }}
            />
          </div>
        </div>
        <div className="flex items-center">
          <input
            type="checkbox"
            id="active"
            checked={editedPolicy.active}
            onChange={(e) => setEditedPolicy({ ...editedPolicy, active: e.target.checked })}
            className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
          />
          <label htmlFor="active" className="ml-2 block text-sm text-gray-900">
            Active
          </label>
        </div>
      </div>
    </div>
  );
}
