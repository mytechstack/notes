import type { Policy } from '../types/policy';

interface PolicyListProps {
  policies: Policy[];
  onSelectPolicy: (policy: Policy) => void;
  onNewPolicy: () => void;
}

export default function PolicyList({ policies, onSelectPolicy, onNewPolicy }: PolicyListProps) {
  return (
    <div className="bg-white shadow rounded-lg">
      <div className="p-4 border-b border-gray-200">
        <div className="flex justify-between items-center">
          <h2 className="text-lg font-medium">Policies</h2>
          <button
            onClick={onNewPolicy}
            className="px-3 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700"
          >
            New Policy
          </button>
        </div>
      </div>
      <ul className="divide-y divide-gray-200">
        {policies.map((policy) => (
          <li key={policy.id}>
            <button
              onClick={() => onSelectPolicy(policy)}
              className="w-full text-left px-4 py-3 hover:bg-gray-50 focus:outline-none focus:bg-gray-50"
            >
              <div className="flex justify-between items-center">
                <div>
                  <p className="text-sm font-medium text-gray-900">{policy.name}</p>
                  <p className="text-sm text-gray-500">{policy.path}</p>
                </div>
                <div className={`h-2 w-2 rounded-full ${policy.active ? 'bg-green-400' : 'bg-red-400'}`} />
              </div>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
