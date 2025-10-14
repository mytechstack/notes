-- Main schema and data initialization

CREATE TABLE IF NOT EXISTS policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    path VARCHAR(500) NOT NULL UNIQUE,
    content TEXT NOT NULL,
    version INTEGER DEFAULT 1,
    metadata JSONB DEFAULT '{}',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255) DEFAULT 'system'
);

CREATE TABLE IF NOT EXISTS policy_versions (
    id SERIAL PRIMARY KEY,
    policy_id INTEGER REFERENCES policies(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255) DEFAULT 'system',
    UNIQUE(policy_id, version)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_policies_active ON policies(active);
CREATE INDEX IF NOT EXISTS idx_policies_updated_at ON policies(updated_at);
CREATE INDEX IF NOT EXISTS idx_policies_path ON policies(path);

-- Triggers for auto-updating timestamps and versioning
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_policies_updated_at
    BEFORE UPDATE ON policies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Sample data
INSERT INTO policies (name, path, content, metadata) VALUES 
('rbac', 'rbac/rbac.rego', 
'package rbac

import rego.v1

allow if {
    user_has_role(input.user, input.required_role)
}

user_has_role(user, role) if {
    role in user.roles
}

default allow := false
', 
'{"description": "Role-based access control", "tags": ["rbac", "security"]}');

SELECT 'Schema and sample data initialized successfully!' as message;