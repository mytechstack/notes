import React, { useState } from 'react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, ReferenceLine, Legend } from 'recharts';

const VisualCompleteSparkline = () => {
  const [viewMode, setViewMode] = useState('overlay'); // 'overlay' or 'separate'
  
  // Sample data - you can replace these with your actual values
  const data = [
    { 
      period: 'Jan 1-15', 
      production: 2.5, 
      performance: 2.2, 
      prodP95: 3.2, 
      prodP99: 4.1,
      perfP95: 2.9,
      perfP99: 3.5,
      index: 0 
    },
    { 
      period: 'Jan 16-31', 
      production: 2.3, 
      performance: 2.0, 
      prodP95: 3.0, 
      prodP99: 3.8,
      perfP95: 2.7,
      perfP99: 3.2,
      index: 1 
    },
    { 
      period: 'Feb 1-15', 
      production: 2.1, 
      performance: 1.9, 
      prodP95: 2.8, 
      prodP99: 3.5,
      perfP95: 2.5,
      perfP99: 3.0,
      index: 2 
    },
    { 
      period: 'Feb 16-28', 
      production: 2.4, 
      performance: 2.1, 
      prodP95: 3.1, 
      prodP99: 3.9,
      perfP95: 2.8,
      perfP99: 3.3,
      index: 3 
    },
    { 
      period: 'Mar 1-15', 
      production: 2.0, 
      performance: 1.8, 
      prodP95: 2.7, 
      prodP99: 3.3,
      perfP95: 2.4,
      perfP99: 2.9,
      index: 4 
    }
  ];

  // Calculate trendlines for both systems
  const calculateTrend = (dataKey) => {
    const n = data.length;
    const sumX = data.reduce((sum, item) => sum + item.index, 0);
    const sumY = data.reduce((sum, item) => sum + item[dataKey], 0);
    const sumXY = data.reduce((sum, item) => sum + item.index * item[dataKey], 0);
    const sumX2 = data.reduce((sum, item) => sum + item.index * item.index, 0);
    
    const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX);
    const intercept = (sumY - slope * sumX) / n;
    
    return { slope, intercept };
  };

  const prodTrend = calculateTrend('production');
  const perfTrend = calculateTrend('performance');

  const dataWithTrends = data.map(item => ({
    ...item,
    prodTrendline: prodTrend.slope * item.index + prodTrend.intercept,
    perfTrendline: perfTrend.slope * item.index + perfTrend.intercept
  }));

  const prodAvg = data.reduce((sum, item) => sum + item.production, 0) / data.length;
  const perfAvg = data.reduce((sum, item) => sum + item.performance, 0) / data.length;
  
  const prodP95Latest = data[data.length - 1].prodP95;
  const prodP99Latest = data[data.length - 1].prodP99;
  const perfP95Latest = data[data.length - 1].perfP95;
  const perfP99Latest = data[data.length - 1].perfP99;
  
  const prodLatest = data[data.length - 1].production;
  const perfLatest = data[data.length - 1].performance;
  
  const prodFirst = data[0].production;
  const perfFirst = data[0].performance;
  
  const prodChange = ((prodLatest - prodFirst) / prodFirst * 100).toFixed(1);
  const perfChange = ((perfLatest - perfFirst) / perfFirst * 100).toFixed(1);
  
  const prodImproving = prodLatest < prodFirst;
  const perfImproving = perfLatest < perfFirst;
  
  // Generate key message based on trends
  const getKeyMessage = () => {
    if (prodImproving && perfImproving) {
      return {
        type: 'success',
        message: 'Both systems showing improvement! Visual complete times are trending downward, indicating better user experience.'
      };
    } else if (!prodImproving && !perfImproving) {
      return {
        type: 'warning',
        message: 'Both systems show degradation in visual complete times. Immediate investigation recommended.'
      };
    } else if (prodImproving && !perfImproving) {
      return {
        type: 'info',
        message: 'Production is improving while Performance system is degrading. Review recent performance environment changes.'
      };
    } else {
      return {
        type: 'info',
        message: 'Performance system improving but Production is degrading. Consider promoting performance optimizations to production.'
      };
    }
  };
  
  const keyMessage = getKeyMessage();

  const OverlayChart = () => (
    <ResponsiveContainer width="100%" height={400}>
      <LineChart data={dataWithTrends} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
        <XAxis 
          dataKey="period" 
          tick={{ fontSize: 12 }}
          stroke="#6b7280"
        />
        <YAxis 
          domain={[1.6, 2.6]}
          tick={{ fontSize: 12 }}
          stroke="#6b7280"
          label={{ value: 'Seconds', angle: -90, position: 'insideLeft' }}
        />
        <Tooltip 
          contentStyle={{ 
            backgroundColor: '#fff', 
            border: '1px solid #e5e7eb',
            borderRadius: '0.5rem',
            padding: '8px 12px'
          }}
        />
        <Legend />
        <ReferenceLine 
          y={prodAvg} 
          stroke="#ef4444" 
          strokeDasharray="3 3"
          strokeOpacity={0.3}
        />
        <ReferenceLine 
          y={perfAvg} 
          stroke="#10b981" 
          strokeDasharray="3 3"
          strokeOpacity={0.3}
        />
        <Line 
          type="monotone" 
          dataKey="prodTrendline" 
          stroke="#ef4444" 
          strokeWidth={2}
          strokeDasharray="5 5"
          dot={false}
          name="Production Trend"
          strokeOpacity={0.5}
        />
        <Line 
          type="monotone" 
          dataKey="perfTrendline" 
          stroke="#10b981" 
          strokeWidth={2}
          strokeDasharray="5 5"
          dot={false}
          name="Performance Trend"
          strokeOpacity={0.5}
        />
        <Line 
          type="monotone" 
          dataKey="production" 
          stroke="#ef4444" 
          strokeWidth={3}
          dot={{ fill: '#ef4444', r: 5 }}
          activeDot={{ r: 7 }}
          name="Production"
        />
        <Line 
          type="monotone" 
          dataKey="performance" 
          stroke="#10b981" 
          strokeWidth={3}
          dot={{ fill: '#10b981', r: 5 }}
          activeDot={{ r: 7 }}
          name="Performance"
        />
      </LineChart>
    </ResponsiveContainer>
  );

  const SeparateCharts = () => (
    <div className="space-y-6">
      <div>
        <h3 className="text-lg font-semibold text-gray-700 mb-2">Production System</h3>
        <ResponsiveContainer width="100%" height={250}>
          <LineChart data={dataWithTrends} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
            <XAxis dataKey="period" tick={{ fontSize: 12 }} stroke="#6b7280" />
            <YAxis domain={[1.8, 2.6]} tick={{ fontSize: 12 }} stroke="#6b7280" />
            <Tooltip contentStyle={{ backgroundColor: '#fff', border: '1px solid #e5e7eb', borderRadius: '0.5rem' }} />
            <ReferenceLine y={prodAvg} stroke="#9ca3af" strokeDasharray="3 3" label={{ value: 'Avg', position: 'right' }} />
            <Line type="monotone" dataKey="prodTrendline" stroke="#f59e0b" strokeWidth={2} strokeDasharray="5 5" dot={false} />
            <Line type="monotone" dataKey="production" stroke="#ef4444" strokeWidth={3} dot={{ fill: '#ef4444', r: 5 }} />
          </LineChart>
        </ResponsiveContainer>
      </div>
      
      <div>
        <h3 className="text-lg font-semibold text-gray-700 mb-2">Performance System</h3>
        <ResponsiveContainer width="100%" height={250}>
          <LineChart data={dataWithTrends} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
            <XAxis dataKey="period" tick={{ fontSize: 12 }} stroke="#6b7280" />
            <YAxis domain={[1.6, 2.2]} tick={{ fontSize: 12 }} stroke="#6b7280" />
            <Tooltip contentStyle={{ backgroundColor: '#fff', border: '1px solid #e5e7eb', borderRadius: '0.5rem' }} />
            <ReferenceLine y={perfAvg} stroke="#9ca3af" strokeDasharray="3 3" label={{ value: 'Avg', position: 'right' }} />
            <Line type="monotone" dataKey="perfTrendline" stroke="#f59e0b" strokeWidth={2} strokeDasharray="5 5" dot={false} />
            <Line type="monotone" dataKey="performance" stroke="#10b981" strokeWidth={3} dot={{ fill: '#10b981', r: 5 }} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <div className="bg-white rounded-lg shadow-lg p-6">
        <div className="flex justify-between items-center mb-2">
          <h2 className="text-2xl font-bold text-gray-800">Visual Complete Time Comparison</h2>
          <div className="flex gap-2">
            <button
              onClick={() => setViewMode('overlay')}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                viewMode === 'overlay' 
                  ? 'bg-blue-600 text-white' 
                  : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
              }`}
            >
              Overlay View
            </button>
            <button
              onClick={() => setViewMode('separate')}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                viewMode === 'separate' 
                  ? 'bg-blue-600 text-white' 
                  : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
              }`}
            >
              Separate Charts
            </button>
          </div>
        </div>
        <p className="text-gray-600 mb-6">Production vs Performance System - Bi-monthly metrics</p>
        
        <div className={`mb-6 p-4 rounded-lg border-l-4 ${
          keyMessage.type === 'success' ? 'bg-green-50 border-green-500' :
          keyMessage.type === 'warning' ? 'bg-red-50 border-red-500' :
          'bg-blue-50 border-blue-500'
        }`}>
          <div className="flex items-start gap-3">
            <div className={`text-2xl ${
              keyMessage.type === 'success' ? 'text-green-600' :
              keyMessage.type === 'warning' ? 'text-red-600' :
              'text-blue-600'
            }`}>
              {keyMessage.type === 'success' ? '✓' : keyMessage.type === 'warning' ? '⚠' : 'ℹ'}
            </div>
            <div>
              <div className="font-semibold text-gray-800 mb-1">Key Insight</div>
              <div className="text-gray-700">{keyMessage.message}</div>
            </div>
          </div>
        </div>
        
        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="bg-red-50 rounded-lg p-4 border-2 border-red-200">
            <div className="flex justify-between items-start mb-3">
              <div className="text-sm text-gray-600 font-medium">Production System</div>
              <div className="flex items-center gap-2">
                <div className={`text-2xl ${prodImproving ? 'text-green-600' : 'text-red-600'}`}>
                  {prodImproving ? '↓' : '↑'}
                </div>
                <div className={`text-xs font-semibold px-2 py-1 rounded ${
                  prodImproving ? 'bg-green-200 text-green-800' : 'bg-red-200 text-red-800'
                }`}>
                  {prodChange}%
                </div>
              </div>
            </div>
            <div className="flex justify-between items-end">
              <div>
                <div className="text-xs text-gray-500">Latest (Median)</div>
                <div className="text-2xl font-bold text-red-600">{prodLatest}s</div>
              </div>
              <div>
                <div className="text-xs text-gray-500">P95</div>
                <div className="text-xl font-semibold text-red-700">{prodP95Latest}s</div>
              </div>
              <div>
                <div className="text-xs text-gray-500">P99</div>
                <div className="text-xl font-semibold text-red-800">{prodP99Latest}s</div>
              </div>
            </div>
            <div className="mt-3 h-2 bg-gray-200 rounded-full overflow-hidden">
              <div 
                className={`h-full transition-all ${prodImproving ? 'bg-green-500' : 'bg-red-500'}`}
                style={{ width: `${Math.abs(parseFloat(prodChange)) * 5}%`, maxWidth: '100%' }}
              ></div>
            </div>
          </div>
          
          <div className="bg-green-50 rounded-lg p-4 border-2 border-green-200">
            <div className="flex justify-between items-start mb-3">
              <div className="text-sm text-gray-600 font-medium">Performance System</div>
              <div className="flex items-center gap-2">
                <div className={`text-2xl ${perfImproving ? 'text-green-600' : 'text-red-600'}`}>
                  {perfImproving ? '↓' : '↑'}
                </div>
                <div className={`text-xs font-semibold px-2 py-1 rounded ${
                  perfImproving ? 'bg-green-200 text-green-800' : 'bg-red-200 text-red-800'
                }`}>
                  {perfChange}%
                </div>
              </div>
            </div>
            <div className="flex justify-between items-end">
              <div>
                <div className="text-xs text-gray-500">Latest (Median)</div>
                <div className="text-2xl font-bold text-green-600">{perfLatest}s</div>
              </div>
              <div>
                <div className="text-xs text-gray-500">P95</div>
                <div className="text-xl font-semibold text-green-700">{perfP95Latest}s</div>
              </div>
              <div>
                <div className="text-xs text-gray-500">P99</div>
                <div className="text-xl font-semibold text-green-800">{perfP99Latest}s</div>
              </div>
            </div>
            <div className="mt-3 h-2 bg-gray-200 rounded-full overflow-hidden">
              <div 
                className={`h-full transition-all ${perfImproving ? 'bg-green-500' : 'bg-red-500'}`}
                style={{ width: `${Math.abs(parseFloat(perfChange)) * 5}%`, maxWidth: '100%' }}
              ></div>
            </div>
          </div>
        </div>

        {viewMode === 'overlay' ? <OverlayChart /> : <SeparateCharts />}

        <div className="mt-6 overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b-2 border-gray-200">
                <th className="text-left py-2 px-3 text-gray-600">Period</th>
                <th className="text-center py-2 px-3 text-red-600">Prod Median</th>
                <th className="text-center py-2 px-3 text-red-700">Prod P95</th>
                <th className="text-center py-2 px-3 text-red-800">Prod P99</th>
                <th className="text-center py-2 px-3 text-green-600">Perf Median</th>
                <th className="text-center py-2 px-3 text-green-700">Perf P95</th>
                <th className="text-center py-2 px-3 text-green-800">Perf P99</th>
              </tr>
            </thead>
            <tbody>
              {data.map((item, index) => (
                <tr key={index} className="border-b border-gray-100">
                  <td className="py-2 px-3 font-medium text-gray-700">{item.period}</td>
                  <td className="py-2 px-3 text-center font-semibold text-red-600">{item.production}s</td>
                  <td className="py-2 px-3 text-center font-semibold text-red-700">{item.prodP95}s</td>
                  <td className="py-2 px-3 text-center font-semibold text-red-800">{item.prodP99}s</td>
                  <td className="py-2 px-3 text-center font-semibold text-green-600">{item.performance}s</td>
                  <td className="py-2 px-3 text-center font-semibold text-green-700">{item.perfP95}s</td>
                  <td className="py-2 px-3 text-center font-semibold text-green-800">{item.perfP99}s</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default VisualCompleteSparkline;