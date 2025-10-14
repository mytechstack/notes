#!/usr/bin/env python3
"""
Policy Loader - Loads Rego policies from files into PostgreSQL database
"""

import os
import sys
import json
import time
import logging
from pathlib import Path
from typing import Dict, Any, List, Optional

import psycopg2
from psycopg2.extras import RealDictCursor

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class PolicyLoader:
    def __init__(self, database_url: str, policy_dir: str = "/app/policies"):
        self.database_url = database_url
        self.policy_dir = Path(policy_dir)
        self.conn = None
        
    def connect_db(self, max_retries: int = 30) -> None:
        """Connect to PostgreSQL with retry logic"""
        for attempt in range(max_retries):
            try:
                self.conn = psycopg2.connect(
                    self.database_url,
                    cursor_factory=RealDictCursor
                )
                logger.info("Connected to database successfully")
                return
            except psycopg2.OperationalError as e:
                logger.warning(f"Database connection attempt {attempt + 1} failed: {e}")
                if attempt < max_retries - 1:
                    time.sleep(2)
                else:
                    raise
                    
    def extract_policy_metadata(self, content: str) -> Dict[str, Any]:
        """Extract metadata from Rego file comments"""
        metadata = {}
        lines = content.split('\n')
        
        for line in lines:
            line = line.strip()
            if line.startswith('#'):
                if 'TITLE:' in line:
                    metadata['title'] = line.split('TITLE:', 1)[1].strip()
                elif 'DESCRIPTION:' in line:
                    metadata['description'] = line.split('DESCRIPTION:', 1)[1].strip()
                elif 'TAGS:' in line:
                    tags = line.split('TAGS:', 1)[1].strip()
                    metadata['tags'] = [t.strip() for t in tags.split(',')]
                elif 'VERSION:' in line:
                    metadata['policy_version'] = line.split('VERSION:', 1)[1].strip()
                elif 'AUTHOR:' in line:
                    metadata['author'] = line.split('AUTHOR:', 1)[1].strip()
        
        # Extract package name
        for line in lines:
            if line.strip().startswith('package '):
                package = line.strip().split('package ', 1)[1]
                metadata['package'] = package
                break
                
        return metadata

    def load_policy_file(self, file_path: Path) -> Optional[Dict[str, Any]]:
        """Load a single policy file"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
            
            if not content.strip():
                logger.warning(f"Skipping empty file: {file_path}")
                return None
                
            metadata = self.extract_policy_metadata(content)
            relative_path = file_path.relative_to(self.policy_dir)
            policy_name = file_path.stem
            
            if 'package' in metadata:
                package_parts = metadata['package'].split('.')
                policy_name = '_'.join(package_parts)
            
            return {
                'name': policy_name,
                'path': str(relative_path),
                'content': content,
                'metadata': metadata
            }
            
        except Exception as e:
            logger.error(f"Error loading policy file {file_path}: {e}")
            return None

    def load_policies_from_directory(self) -> List[Dict[str, Any]]:
        """Load all .rego files from the policy directory"""
        policies = []
        
        if not self.policy_dir.exists():
            logger.warning(f"Policy directory does not exist: {self.policy_dir}")
            return policies
            
        logger.info(f"Loading policies from: {self.policy_dir}")
        rego_files = list(self.policy_dir.rglob("*.rego"))
        logger.info(f"Found {len(rego_files)} .rego files")
        
        for rego_file in rego_files:
            logger.info(f"Processing: {rego_file}")
            policy = self.load_policy_file(rego_file)
            if policy:
                policies.append(policy)
                
        return policies

    def save_policy_to_db(self, policy: Dict[str, Any]) -> bool:
        """Save a single policy to the database"""
        try:
            with self.conn.cursor() as cursor:
                cursor.execute("""
                    INSERT INTO policies (name, path, content, metadata, created_by, active)
                    VALUES (%(name)s, %(path)s, %(content)s, %(metadata)s, 'policy-loader', true)
                    ON CONFLICT (name) DO UPDATE SET
                        content = EXCLUDED.content,
                        metadata = EXCLUDED.metadata,
                        updated_at = NOW(),
                        active = true
                    RETURNING id, version
                """, {
                    'name': policy['name'],
                    'path': policy['path'],
                    'content': policy['content'],
                    'metadata': json.dumps(policy['metadata'])
                })
                
                result = cursor.fetchone()
                if result:
                    logger.info(f"Saved policy '{policy['name']}' (ID: {result['id']}, Version: {result['version']})")
                    return True
                    
        except Exception as e:
            logger.error(f"Error saving policy '{policy['name']}': {e}")
            self.conn.rollback()
            return False
            
        return False

    def get_database_stats(self) -> Dict[str, Any]:
        """Get database statistics"""
        try:
            with self.conn.cursor() as cursor:
                cursor.execute("""
                    SELECT 
                        COUNT(*) as total_policies,
                        COUNT(*) FILTER (WHERE active = true) as active_policies,
                        COUNT(*) FILTER (WHERE active = false) as inactive_policies,
                        MAX(updated_at) as last_updated
                    FROM policies
                """)
                result = cursor.fetchone()
                return dict(result) if result else {}
        except Exception as e:
            logger.error(f"Error getting database stats: {e}")
            return {}

    def load_all_policies(self) -> None:
        """Load all policies from directory to database"""
        logger.info("Starting policy loading process...")
        
        initial_stats = self.get_database_stats()
        logger.info(f"Initial database state: {initial_stats}")
        
        policies = self.load_policies_from_directory()
        
        if not policies:
            logger.warning("No policies found to load")
            return
            
        successful = 0
        failed = 0
        
        for policy in policies:
            if self.save_policy_to_db(policy):
                successful += 1
            else:
                failed += 1
                
        try:
            self.conn.commit()
            logger.info("Transaction committed successfully")
        except Exception as e:
            logger.error(f"Error committing transaction: {e}")
            self.conn.rollback()
            
        final_stats = self.get_database_stats()
        
        logger.info("Policy loading completed!")
        logger.info(f"Successfully loaded: {successful} policies")
        logger.info(f"Failed to load: {failed} policies")
        logger.info(f"Final database state: {final_stats}")

    def close(self):
        """Close database connection"""
        if self.conn:
            self.conn.close()
            logger.info("Database connection closed")

def main():
    """Main entry point"""
    database_url = os.environ.get('DATABASE_URL')
    if not database_url:
        logger.error("DATABASE_URL environment variable is required")
        sys.exit(1)
        
    policy_dir = os.environ.get('POLICY_DIR', '../policies')
    
    loader = PolicyLoader(database_url, policy_dir)
    
    try:
        loader.connect_db()
        loader.load_all_policies()
        logger.info("Policy loader completed successfully")
        
    except Exception as e:
        logger.error(f"Policy loader failed: {e}")
        sys.exit(1)
    finally:
        loader.close()

if __name__ == "__main__":
    main()