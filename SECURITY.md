# Security Policy

## Reporting a Vulnerability

We take security very seriously. If you believe you've found a security vulnerability in MiBeeHive, please follow these steps:

### 1. Private Disclosure
- **Do not disclose publicly** - Keep vulnerabilities private until they are resolved
- **Contact us privately** via GitHub Issues with the "Security" label
- **Provide sufficient detail** to reproduce the issue

### 2. Vulnerability Information
Include in your report:
- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Affected versions
- Your contact information for follow-up

### 3. Timeline
We will follow this timeline:
- **Initial response**: Within 48 hours
- **Fix development**: Within 14 days for critical issues
- **Public disclosure**: After a fix is available and users have been notified

## Supported Versions

We currently support the latest stable version of MiBeeHive.

| Version | Supported | Latest |
|---------|-----------|--------|
| 1.0.x   | ✅       | ✅     |
| < 1.0.0 | ❌       | ❌     |

## Security Features

MiBeeHive includes several security features:

### Authentication
- **JWT Authentication**: Secure token-based authentication for all admin endpoints
- **Password Management**: bcrypt password hashing with change requirements
- **Session Management**: Automatic token expiration and refresh

### Access Control
- **Role-based Access**: Anonymous read-only + admin read-write for WebDAV
- **Endpoint Protection**: All admin endpoints require JWT authentication
- **Public Endpoints**: PXE endpoints remain public for network boot functionality

### Network Security
- **TLS Encryption**: HTTPS support with self-signed certificates
- **Security Headers**: CSP, X-Frame-Options, X-Content-Type-Options headers
- **Rate Limiting**: Protection against brute force and DoS attacks
- **Input Validation**: All user inputs are validated and sanitized

### Data Protection
- **Secure Storage**: SQLite database with file-based security
- **Configuration Security**: Sensitive data stored separately from config
- **Backup Security**: Encrypted backup options for sensitive data

## Common Security Practices

### For Users
1. **Change default passwords** immediately after first login
2. **Use strong JWT secrets** - avoid default values
3. **Configure rate limiting** appropriate for your environment
4. **Monitor logs** for suspicious activity
5. **Keep backups** of configuration and data

### For Administrators
1. **Restrict network access** to necessary ports only (9090, 9443)
2. **Use firewall rules** to limit access to admin endpoints
3. **Regular security audits** of the system
4. **Monitor system resources** for unusual activity
5. **Update regularly** when security patches are released

## Security Best Practices

### WebDAV Security
- Use HTTPS for all WebDAV traffic
- Consider additional authentication layers for sensitive data
- Regular audit of shared files and permissions
- Monitor for unusual access patterns

### Container Security
- Only use trusted container images
- Regular updates of base images and applications
- Monitor container resource usage and behavior
- Implement network isolation where possible

### File System Security
- Regular backup of important data
- Monitor disk space usage to prevent denial of service
- Audit downloaded files for integrity
- Implement appropriate file permissions

## Vulnerability Disclosure

### What we look for
- Authentication bypass vulnerabilities
- Authorization flaws
- Input validation issues
- Data exposure vulnerabilities
- Denial of service vulnerabilities
- Configuration security issues

### What we don't cover
- Third-party dependency vulnerabilities (we monitor and update these)
- Issues in operating system or hardware
- Social engineering attacks
- Physical security breaches

## Contact Information

For security questions or vulnerability reports:
- GitHub Issues: [Create a security report](https://github.com/Mi-Bee-Studio/mibeehive/issues/new?labels=Security&template=security_report.md)

## Security Acknowledgments

We thank the security research community for their help in making MiBeeHive more secure.