import { useState, useEffect } from 'react';
import axios from 'axios';
import PolicyEditor from './components/PolicyEditor';
import PolicyList from './components/PolicyList';
import type { Policy } from './types/policy';


function App() {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [selectedPolicy, setSelectedPolicy] = useState<Policy | null>(null);

  useEffect(() => {
    fetchPolicies();
  }, []);

  const fetchPolicies = async () => {
    try {
      const response = await axios.get('http://localhost:8080/policies');
      setPolicies(response.data);
    } catch (error) {
      console.error('Error fetching policies:', error);
    }
  };

  const handleSavePolicy = async (policy: Policy) => {
    try {
      if (policy.id && policy.id !== '') {
        await axios.put(`http://localhost:8080/policies/${policy.id}`, policy);
      } else {
        await axios.post('http://localhost:8080/policies', policy);
      }
      fetchPolicies();
    } catch (error) {
      console.error('Error saving policy:', error);
    }
  };

  return (
    <>
      <div>
        <a href="https://vite.dev" target="_blank">
         
        </a>
        <a href="https://react.dev" target="_blank">
          
        </a>
      </div>
      <h1>Vite + React</h1>
      <div className="min-h-screen bg-gray-100">
        <nav className="bg-white shadow-sm">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <h1 className="text-xl font-semibold">OPA Policy Manager</h1>
              </div>
            </div>
          </div>
        </nav>

        <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
          <div className="grid grid-cols-12 gap-6">
            <div className="col-span-4">
              <PolicyList 
                policies={policies}
                onSelectPolicy={setSelectedPolicy}
                onNewPolicy={() => setSelectedPolicy({
                  id: '',
                  name: '',
                  path: '',
                  content: '',
                  active: true
                })}
              />
            </div>
            <div className="col-span-8">
              {selectedPolicy && (
                <PolicyEditor
                  policy={selectedPolicy}
                  onSave={handleSavePolicy}
                  onCancel={() => setSelectedPolicy(null)}
                />
              )}
            </div>
          </div>
        </main>
      </div>
    </>
  )
}

export default App
